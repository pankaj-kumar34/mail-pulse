package main

import (
	"slices"
	"reflect"
	"testing"
)

func TestLooksRandom(t *testing.T) {
	garbage := []string{"xk7fj29dqmz", "q9w8e7r5t4y", "ab12cd34ef56", "jsmith660127", "user20240x9z"}
	for _, s := range garbage {
		if !looksRandom(s) {
			t.Errorf("looksRandom(%q) = false, want true", s)
		}
	}
	real := []string{"john.smith", "sarah.chen", "ops-team", "notifications", "pankaj.kumar", "support.team2", "jdoe"}
	for _, s := range real {
		if looksRandom(s) {
			t.Errorf("looksRandom(%q) = true, want false", s)
		}
	}
}

func TestAddrFlags(t *testing.T) {
	if f := addrFlags("dps-uat@openpages.local"); len(f) == 0 || f[0] != "reserved-tld" {
		t.Errorf("addrFlags reserved-tld = %v", f)
	}
	if f := addrFlags("xk7fj29dqmz@mail.com"); len(f) == 0 || f[0] != "random-localpart" {
		t.Errorf("addrFlags random-localpart = %v", f)
	}
	if f := addrFlags("john.smith@ibm.com"); len(f) != 0 {
		t.Errorf("addrFlags clean addr = %v, want none", f)
	}
	if f := addrFlags("no-at-sign"); len(f) != 0 {
		t.Errorf("addrFlags no domain = %v, want none", f)
	}
}

func TestQueryCache(t *testing.T) {
	s := newLogStore()
	s.add(&ParsedLogEntry{ID: "a", Service: "smtp", Timestamp: 1000, QueueID: "q1", From: "x@y.com"})

	m1 := s.getMetrics(0)
	if !reflect.DeepEqual(m1, s.computeMetrics(0)) {
		t.Fatal("cached result differs from fresh compute")
	}
	if len(s.queryCache) == 0 {
		t.Fatal("query cache not populated after getMetrics")
	}

	// duplicate add — content unchanged, cache must survive
	s.add(&ParsedLogEntry{ID: "a", Service: "smtp", Timestamp: 1000})
	if len(s.queryCache) == 0 {
		t.Fatal("dup add should not invalidate cache")
	}

	// new content — generation bumped, cache dropped, fresh recompute
	s.add(&ParsedLogEntry{ID: "b", Service: "smtp", Timestamp: 2000, IsError: true})
	if len(s.queryCache) != 0 {
		t.Fatal("cache should be empty after content change")
	}
	if !reflect.DeepEqual(s.getMetrics(0), s.computeMetrics(0)) {
		t.Fatal("post-invalidation result differs from fresh compute")
	}

	// different args are distinct cache entries, not cross-hits
	before := len(s.queryCache)
	s.getMetrics(1500)
	if len(s.queryCache) != before+1 {
		t.Fatal("getMetrics(1500) collided with getMetrics(0) cache key")
	}
}

func TestClassifyBucket(t *testing.T) {
	cases := []struct{ reason, status, dsn, want string }{
		{"Connection refused", "deferred", "", "Connection / network"},
		{"Policy block", "deferred", "4.7.1", "Block / reputation"},
		{"IP/domain blocked", "bounced", "5.7.1", "Block / reputation"},
		{"Unknown user (5.1.1)", "bounced", "5.1.1", "Hard bounce"},
		{"Rate limited", "deferred", "4.7.0", "Soft / temporary"},
		{"Unknown recipient", "deferred", "4.1.1", "Soft / temporary"},
		{"Other", "deferred", "4.4.2", "Soft / temporary"},
		{"Other", "bounced", "5.0.0", "Hard bounce"},
		{"", "deferred", "", "Other"},
	}
	for _, c := range cases {
		if got := classifyBucket(c.reason, c.status, c.dsn); got != c.want {
			t.Errorf("classifyBucket(%q,%q,%q) = %q, want %q", c.reason, c.status, c.dsn, got, c.want)
		}
	}
	// every reason produced by the rules must map to a bucket — no stray keys
	for _, r := range deferRules {
		if reasonBuckets[r.reason] == "" {
			t.Errorf("deferRules reason %q has no bucket", r.reason)
		}
	}
	for _, r := range bounceRules {
		if reasonBuckets[r.reason] == "" {
			t.Errorf("bounceRules reason %q has no bucket", r.reason)
		}
	}
}

func TestJourney(t *testing.T) {
	s := newLogStore()
	s.add(&ParsedLogEntry{ID: "1", Timestamp: 1000, Service: "qmgr", QueueID: "ABC1234567", From: "bob@x.com", Status: "active"})
	s.add(&ParsedLogEntry{ID: "2", Timestamp: 2000, Service: "smtp", QueueID: "ABC1234567", To: "qa-user@mail.com", Status: "deferred", DSN: "4.7.1", SMTPMsg: "blocked"})
	s.add(&ParsedLogEntry{ID: "3", Timestamp: 3000, Service: "smtp", QueueID: "ABC1234567", To: "qa-user@mail.com", Status: "sent", DSN: "2.0.0"})
	s.add(&ParsedLogEntry{ID: "4", Timestamp: 2500, Service: "smtpd", To: "qa-user@mail.com", Status: "rejected", DSN: "5.7.1", SMTPMsg: "RCPT rejected"})

	// queue-ID mode, lowercase input normalised
	j := s.getJourney("abc1234567")
	if len(j.Queues) != 1 || j.Queues[0].QueueID != "ABC1234567" {
		t.Fatalf("queue-ID search: got %d queues, want 1 for ABC1234567", len(j.Queues))
	}
	q := j.Queues[0]
	if q.From != "bob@x.com" || q.To != "qa-user@mail.com" || q.FinalStatus != "sent" {
		t.Errorf("journey header = %+v", q)
	}
	if len(q.Events) != 3 {
		t.Fatalf("event count = %d, want 3", len(q.Events))
	}
	if q.Events[0].Kind != "queued" || q.Events[1].Kind != "delivery" || q.Events[1].Status != "deferred" {
		t.Errorf("event kinds/status wrong: %+v", q.Events)
	}
	if q.Events[1].Timestamp > q.Events[2].Timestamp {
		t.Error("events not sorted chronologically")
	}

	// recipient mode: picks up the NOQUEUE policy event and the queue
	j = s.getJourney("qa-user@mail.com")
	if len(j.Queues) != 2 {
		t.Fatalf("recipient search: got %d queues, want 2 (NOQUEUE + ABC1234567)", len(j.Queues))
	}
	foundPolicy := false
	for _, qq := range j.Queues {
		if qq.QueueID == "" && len(qq.Events) == 1 && qq.Events[0].Kind == "policy" {
			foundPolicy = true
		}
	}
	if !foundPolicy {
		t.Error("NOQUEUE policy event missing from recipient journey")
	}

	// no match
	if j := s.getJourney("nobody@nowhere.invalid"); len(j.Queues) != 0 {
		t.Errorf("no-match search returned %d queues", len(j.Queues))
	}
	// empty query
	if j := s.getJourney("  "); len(j.Queues) != 0 {
		t.Errorf("empty search returned %d queues", len(j.Queues))
	}
}

func TestGetGarbageSenderJoin(t *testing.T) {
	s := newLogStore()
	// qmgr line carries from=; the deferred delivery line does not — sender must be joined via queue ID
	s.add(&ParsedLogEntry{ID: "q1", Service: "qmgr", Timestamp: 1000, QueueID: "ABC1", From: "app@corp.ibm.com"})
	s.add(&ParsedLogEntry{ID: "d1", Service: "smtp", Timestamp: 2000, QueueID: "ABC1", To: "dps-uat@openpages.local", Status: "deferred"})

	rows := s.getGarbage(0)
	if len(rows) != 1 {
		t.Fatalf("getGarbage rows = %d, want 1", len(rows))
	}
	if len(rows[0].Senders) != 1 || rows[0].Senders[0] != "app@corp.ibm.com" {
		t.Errorf("senders = %v, want [app@corp.ibm.com] joined via queueFrom", rows[0].Senders)
	}
}

func TestGetGarbageRandomLocalpartNeedsBounce(t *testing.T) {
	s := newLogStore()
	// real student-ID mailbox that delivered: shape guess downgraded to "unverified", not dropped
	s.add(&ParsedLogEntry{ID: "a1", Service: "smtp", Timestamp: 1000, QueueID: "Q1", To: "2024693536@student.uitm.edu.my", Status: "sent"})
	rows := s.getGarbage(0)
	if len(rows) != 1 || !slices.Contains(rows[0].Flags, "unverified") {
		t.Fatalf("delivered random-looking recipient: rows=%d flags=%v, want 1 row [unverified]", len(rows), rows[0].Flags)
	}
	// bounced 5.1.1 to a random-looking address: evidence upgrades it to random-localpart + user-unknown
	s.add(&ParsedLogEntry{ID: "b1", Service: "smtp", Timestamp: 2000, QueueID: "Q2", To: "xk7fj29dqmz@corp.com", Status: "bounced", DSN: "5.1.1"})
	rows = s.getGarbage(0)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	for _, r := range rows {
		if r.To != "xk7fj29dqmz@corp.com" {
			continue
		}
		if !slices.Contains(r.Flags, "random-localpart") || !slices.Contains(r.Flags, "user-unknown") {
			t.Errorf("flags = %v, want random-localpart + user-unknown", r.Flags)
		}
	}
}
