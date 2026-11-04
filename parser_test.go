package main

import (
	"strings"
	"testing"
)

// ──────────────────────────────────────────────
// parsePostfixLog
// ──────────────────────────────────────────────

func TestParsePostfixLog_Sent(t *testing.T) {
	raw := `Nov 03 00:00:04 mail postfix/smtp[129270]: 2EA1F80020: to=<sarah.d.green@ibm.com>, relay=mx0b-001b2d05.pphosted.com[205.220.172.48]:25, delay=1.9, delays=0.21/0.49/0.35/0.83, dsn=2.0.0, status=sent (250 2.0.0 Message accepted)`
	e := parsePostfixLog(raw)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if e.Status != "sent" {
		t.Errorf("Status = %q, want %q", e.Status, "sent")
	}
	if e.To != "sarah.d.green@ibm.com" {
		t.Errorf("To = %q, want sarah.d.green@ibm.com", e.To)
	}
	if e.QueueID != "2EA1F80020" {
		t.Errorf("QueueID = %q, want 2EA1F80020", e.QueueID)
	}
	if e.Delay != 1.9 {
		t.Errorf("Delay = %v, want 1.9", e.Delay)
	}
	if e.Delays == nil {
		t.Fatal("Delays is nil")
	}
	if e.Delays.BeforeQmgr != 0.21 || e.Delays.InQmgr != 0.49 || e.Delays.ConnSetup != 0.35 || e.Delays.Transmission != 0.83 {
		t.Errorf("Delays = %+v, want 0.21/0.49/0.35/0.83", e.Delays)
	}
	if e.IsError || e.IsWarning {
		t.Errorf("sent line should not be error/warning, got IsError=%v IsWarning=%v", e.IsError, e.IsWarning)
	}
}

func TestParsePostfixLog_Deferred(t *testing.T) {
	raw := `Nov 03 00:00:03 mail postfix/smtp[129254]: D9D5E80236: to=<matthew.mcgeary@nutrien.com>, relay=nutrien-com.mail.protection.outlook.com[52.101.60.182]:25, delay=8713, delays=8711/0.32/1.1/0.32, dsn=4.7.500, status=deferred (host said: 451 4.7.500 Server busy)`
	e := parsePostfixLog(raw)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if e.Status != "deferred" {
		t.Errorf("Status = %q, want deferred", e.Status)
	}
	if e.IsError {
		t.Error("deferred line should not be IsError=true")
	}
	if e.RawLog == "" {
		t.Error("deferred line should have RawLog set for reason extraction")
	}
}

func TestParsePostfixLog_Bounced(t *testing.T) {
	raw := `Nov 03 00:00:13 mail postfix/smtp[129263]: 4E84780258: to=<sowji.karumuri+ibmreporting@sysdig.com>, relay=aspmx.l.google.com[172.253.156.26]:25, delay=4.5, delays=0.18/0/4.2/0.18, dsn=5.2.1, status=bounced (host said: 550-5.2.1 The email account that you tried to reach is inactive)`
	e := parsePostfixLog(raw)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if e.Status != "bounced" {
		t.Errorf("Status = %q, want bounced", e.Status)
	}
	if e.IsError {
		t.Error("bounced line should not be IsError=true")
	}
}

func TestParsePostfixLog_BounceWithErrorInDSN(t *testing.T) {
	// "Name service error" in DSN text must NOT set IsError
	raw := `Nov 03 00:49:30 mail postfix/smtp[129619]: D6AAE80590: to=<amolshi1@in.ibm>, relay=none, delay=0.07, delays=0.02/0/0.05/0, dsn=5.4.4, status=bounced (Host or domain name not found. Name service error for name=in.ibm type=AAAA: Host not found)`
	e := parsePostfixLog(raw)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if e.Status != "bounced" {
		t.Errorf("Status = %q, want bounced", e.Status)
	}
	if e.IsError {
		t.Error("bounced line with 'Name service error' in DSN text must not set IsError=true")
	}
}

func TestParsePostfixLog_NDR(t *testing.T) {
	// from=<> means NDR/DSN notification
	raw := `Nov 03 00:00:02 mail postfix/qmgr[129239]: 39551800E8: from=<>, size=4358, nrcpt=1 (queue active)`
	e := parsePostfixLog(raw)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if e.From != "<>" {
		t.Errorf("From = %q, want <>", e.From)
	}
	if e.Status != "active" {
		t.Errorf("Status = %q, want active", e.Status)
	}
}

func TestParsePostfixLog_QmgrActive(t *testing.T) {
	raw := `Nov 03 00:00:02 mail postfix/qmgr[129239]: 3FFD980237: from=<no-reply@insights.ibm.com>, size=1706, nrcpt=1 (queue active)`
	e := parsePostfixLog(raw)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if e.Status != "active" {
		t.Errorf("Status = %q, want active", e.Status)
	}
	if e.From != "no-reply@insights.ibm.com" {
		t.Errorf("From = %q, want no-reply@insights.ibm.com", e.From)
	}
}

func TestParsePostfixLog_Warning(t *testing.T) {
	raw := `Nov 03 00:17:30 mail postfix/smtp[129351]: warning: numeric domain name in resource data of MX record for qa.dev.draios.com: 3.231.185.193`
	e := parsePostfixLog(raw)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if !e.IsWarning {
		t.Error("warning: line should have IsWarning=true")
	}
	if e.IsError {
		t.Error("warning: line should not have IsError=true")
	}
}

func TestParsePostfixLog_Fatal(t *testing.T) {
	raw := `Nov 03 00:17:30 mail postfix/smtp[129351]: fatal: config variable inet_interfaces: unknown host`
	e := parsePostfixLog(raw)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if !e.IsError {
		t.Error("fatal: line should have IsError=true")
	}
}

func TestParsePostfixLog_ErrorNotPrefixed(t *testing.T) {
	// "error" appearing mid-message (not as prefix) should NOT set IsError
	raw := `Nov 03 00:49:30 mail postfix/smtp[129619]: D6AAE80590: to=<a@b.com>, relay=none, delay=0.07, delays=0.02/0/0.05/0, dsn=5.4.4, status=bounced (Name service error for name=b.com)`
	e := parsePostfixLog(raw)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if e.IsError {
		t.Error("mid-message 'error' should not set IsError=true")
	}
}

func TestParsePostfixLog_SkipsIrrelevantServices(t *testing.T) {
	// postfix/cleanup, postfix/pickup etc. should be ignored
	raw := `Nov 03 00:00:01 mail postfix/cleanup[1234]: ABC123: message-id=<foo@bar.com>`
	e := parsePostfixLog(raw)
	if e != nil {
		t.Errorf("expected nil for cleanup line, got %+v", e)
	}
}

func TestParsePostfixLog_ConnectTo(t *testing.T) {
	// "connect to" lines have no queue ID and no status
	raw := `Nov 03 00:00:02 mail postfix/smtp[129245]: connect to insights.ibm.com[2606:4700::6812:1ef7]:25: Network is unreachable`
	e := parsePostfixLog(raw)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if e.Status != "" {
		t.Errorf("connect line should have no Status, got %q", e.Status)
	}
	if e.QueueID != "" {
		t.Errorf("connect line should have no QueueID, got %q", e.QueueID)
	}
	if e.IsError {
		t.Error("connect line should not be IsError")
	}
}

func TestParsePostfixLog_PostfixErrorService(t *testing.T) {
	// postfix/error[...] lines carry status=deferred with dsn=4.4.1
	raw := `Nov 03 00:01:02 mail postfix/error[129292]: B2D2480085: to=<no-reply@insights.ibm.com>, relay=none, delay=56132, delays=56071/60/0/0, dsn=4.4.1, status=deferred (delivery temporarily suspended: connect to insights.ibm.com[2606:4700::6812:1ff7]:25: Network is unreachable)`
	e := parsePostfixLog(raw)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if e.Status != "deferred" {
		t.Errorf("Status = %q, want deferred", e.Status)
	}
	if e.IsError {
		t.Error("postfix/error deferred line should not set IsError=true")
	}
}

func TestParsePostfixLog_UniqueIDs(t *testing.T) {
	// Two lines same timestamp and same PID must get different IDs
	a := `Nov 03 00:00:02 mail postfix/smtp[129251]: connect to insights.ibm.com[2606:4700::6812:1ef7]:25: Network is unreachable`
	b := `Nov 03 00:00:02 mail postfix/smtp[129251]: connect to insights.ibm.com[2606:4700::6812:1ff7]:25: Network is unreachable`
	ea := parsePostfixLog(a)
	eb := parsePostfixLog(b)
	if ea == nil || eb == nil {
		t.Fatal("both entries should parse")
	}
	if ea.ID == eb.ID {
		t.Errorf("two distinct lines got identical ID: %q", ea.ID)
	}
}

// ──────────────────────────────────────────────
// extractDeferReason
// ──────────────────────────────────────────────

func TestExtractDeferReason(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"451 4.7.500 Server busy. Please try again later", "Server busy (M365)"},
		{"connect to host[::1]:25: Network is unreachable", "Network unreachable (IPv6)"},
		{"delivery temporarily suspended: connect to host", "Network unreachable (IPv6)"},
		{"Host or domain name not found. Name service error", "DNS failure"},
		{"Connection refused", "Connection refused"},
		{"connect to host:25: Connection refused", "Connection refused"},
		{"lost connection with host", "Lost connection"},
		{"connection timed out", "Timeout"},
		{"too many connections from", "Rate limited"},
		{"greylisted", "Greylisted"},
		{"TLS handshake failed", "TLS error"},
		{"mailbox full", "Mailbox full"},
		{"some completely unknown reason xyz", "Other"},
		{"", ""},
	}
	for _, c := range cases {
		got := extractDeferReason(c.raw)
		if got != c.want {
			t.Errorf("extractDeferReason(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

// ──────────────────────────────────────────────
// extractBounceReason
// ──────────────────────────────────────────────

func TestExtractBounceReason(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"550 5.2.1 The email account that you tried to reach is inactive", "Inactive account (5.2.1)"},
		{"550 5.1.1 The email account does not exist", "Unknown user (5.1.1)"},
		{"user unknown", "Unknown user (5.1.1)"},
		{"550 5.4.1 Recipient address rejected: Access denied", "Address rejected (5.4.1)"},
		{"550 5.7.1 Message rejected due to policy", "Policy rejection (5.7.1)"},
		{"550 5.2.2 mailbox full", "Mailbox full (5.2.2)"},
		{"blocked by policy", "IP/domain blocked"},
		{"Host or domain name not found. Name service error", "DNS failure"},
		{"550 5.7.26 dmarc policy violation", "Auth/policy failure (5.7.x)"},
		{"some unknown bounce reason", "Other"},
		{"", ""},
	}
	for _, c := range cases {
		got := extractBounceReason(c.raw)
		if got != c.want {
			t.Errorf("extractBounceReason(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

// ──────────────────────────────────────────────
// logStore — NDR filtering
// ──────────────────────────────────────────────

func TestLogStore_NDRExcludedFromMetrics(t *testing.T) {
	s := newLogStore()

	// NDR qmgr line — from=<>
	ndrQmgr := parsePostfixLog(`Nov 03 00:00:02 mail postfix/qmgr[129239]: AABBCC0011: from=<>, size=4358, nrcpt=1 (queue active)`)
	// NDR smtp delivery line — same queue ID, no from= on this line
	ndrSmtp := parsePostfixLog(`Nov 03 00:00:05 mail postfix/smtp[129239]: AABBCC0011: to=<victim@example.com>, relay=mx.example.com[1.2.3.4]:25, delay=1, delays=0/0/0.5/0.5, dsn=2.0.0, status=sent (250 ok)`)
	// Normal sent line
	normal := parsePostfixLog(`Nov 03 00:00:04 mail postfix/smtp[129270]: 2EA1F80020: to=<sarah@ibm.com>, relay=mx.ibm.com[1.2.3.4]:25, delay=1.9, delays=0.21/0.49/0.35/0.83, dsn=2.0.0, status=sent (250 ok)`)

	if ndrQmgr == nil || ndrSmtp == nil || normal == nil {
		t.Fatal("all entries must parse")
	}

	s.add(ndrQmgr)
	s.add(ndrSmtp)
	s.add(normal)

	m := s.getMetrics(0)
	if m.TotalSent != 1 {
		t.Errorf("TotalSent = %d, want 1 (NDR delivery should be excluded)", m.TotalSent)
	}
}

func TestLogStore_DedupByFullLine(t *testing.T) {
	s := newLogStore()

	// Two different lines that share the same timestamp and first 50 chars (old bug)
	a := parsePostfixLog(`Nov 03 00:00:02 mail postfix/smtp[129251]: connect to insights.ibm.com[2606:4700::6812:1ef7]:25: Network is unreachable`)
	b := parsePostfixLog(`Nov 03 00:00:02 mail postfix/smtp[129251]: connect to insights.ibm.com[2606:4700::6812:1ff7]:25: Network is unreachable`)

	if a == nil || b == nil {
		t.Fatal("both must parse")
	}

	s.add(a)
	s.add(b)

	s.mu.RLock()
	count := len(s.logs)
	s.mu.RUnlock()

	if count != 2 {
		t.Errorf("store has %d entries, want 2 (dedup should not collapse distinct lines)", count)
	}
}

func TestLogStore_DedupsIdenticalLine(t *testing.T) {
	s := newLogStore()

	raw := `Nov 03 00:00:04 mail postfix/smtp[129270]: 2EA1F80020: to=<sarah@ibm.com>, relay=mx.ibm.com[1.2.3.4]:25, delay=1.9, delays=0.21/0.49/0.35/0.83, dsn=2.0.0, status=sent (250 ok)`
	e := parsePostfixLog(raw)
	if e == nil {
		t.Fatal("must parse")
	}

	s.add(e)
	s.add(e) // exact same entry

	s.mu.RLock()
	count := len(s.logs)
	s.mu.RUnlock()

	if count != 1 {
		t.Errorf("store has %d entries, want 1 (identical line should be deduped)", count)
	}
}

// ──────────────────────────────────────────────
// extractDomain
// ──────────────────────────────────────────────

func TestExtractDomain(t *testing.T) {
	cases := []struct{ email, want string }{
		{"user@example.com", "example.com"},
		{"user@sub.domain.co.uk", "sub.domain.co.uk"},
		{"nodomain", "unknown"},
		{"", "unknown"},
	}
	for _, c := range cases {
		got := extractDomain(c.email)
		if got != c.want {
			t.Errorf("extractDomain(%q) = %q, want %q", c.email, got, c.want)
		}
	}
}

// ──────────────────────────────────────────────
// matchesQuery (mailqueue)
// ──────────────────────────────────────────────

func TestMatchesQuery(t *testing.T) {
	e := MailQueueEntry{
		QueueID:   "ABC123",
		Sender:    "sender@example.com",
		QueueName: "deferred",
		Recipients: []MailQueueRecipient{
			{Address: "recipient@target.com"},
		},
	}
	if !matchesQuery(e, "abc123") {
		t.Error("should match queue ID case-insensitively")
	}
	if !matchesQuery(e, "example.com") {
		t.Error("should match sender domain")
	}
	if !matchesQuery(e, "target.com") {
		t.Error("should match recipient")
	}
	if !matchesQuery(e, "deferred") {
		t.Error("should match queue name")
	}
	if matchesQuery(e, "nomatch") {
		t.Error("should not match unrelated query")
	}
}

// ──────────────────────────────────────────────
// parseMailqStream
// ──────────────────────────────────────────────

func TestParseMailqStream(t *testing.T) {
	body := `pod relay-pod-abc
{"queue_name":"deferred","queue_id":"AAA111","arrival_time":1000,"message_size":1024,"forced_expire":false,"sender":"from@example.com","recipients":[{"address":"to@example.com"}]}
{"queue_name":"active","queue_id":"BBB222","arrival_time":2000,"message_size":512,"forced_expire":false,"sender":"x@y.com","recipients":[]}

`
	podID, entries, err := parseMailqStream(strings.NewReader(body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if podID != "relay-pod-abc" {
		t.Errorf("podID = %q, want relay-pod-abc", podID)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}
	if entries[0].QueueID != "AAA111" {
		t.Errorf("entries[0].QueueID = %q, want AAA111", entries[0].QueueID)
	}
	if entries[1].QueueID != "BBB222" {
		t.Errorf("entries[1].QueueID = %q, want BBB222", entries[1].QueueID)
	}
}

// ──────────────────────────────────────────────
// getRecipientDrilldown
// ──────────────────────────────────────────────

func TestGetRecipientDrilldown(t *testing.T) {
	s := newLogStore()

	lines := []string{
		// 3 bounces to sowji@sysdig.com
		`Nov 03 00:00:13 mail postfix/smtp[1]: AAA000000A: to=<sowji@sysdig.com>, relay=aspmx.l.google.com[1.2.3.4]:25, delay=4.5, delays=0.18/0/4.2/0.18, dsn=5.2.1, status=bounced (550 5.2.1 inactive)`,
		`Nov 03 00:00:19 mail postfix/smtp[1]: BBB000000B: to=<sowji@sysdig.com>, relay=aspmx.l.google.com[1.2.3.4]:25, delay=4.5, delays=0.16/0/4.1/0.21, dsn=5.2.1, status=bounced (550 5.2.1 inactive)`,
		`Nov 03 00:00:24 mail postfix/smtp[1]: CCC000000C: to=<sowji@sysdig.com>, relay=aspmx.l.google.com[1.2.3.4]:25, delay=4.4, delays=0.17/0/4.1/0.19, dsn=5.2.1, status=bounced (550 5.2.1 inactive)`,
		// 1 sent to other@sysdig.com
		`Nov 03 00:00:30 mail postfix/smtp[1]: DDD000000D: to=<other@sysdig.com>, relay=aspmx.l.google.com[1.2.3.4]:25, delay=1.0, delays=0.1/0/0.5/0.4, dsn=2.0.0, status=sent (250 ok)`,
	}
	for _, l := range lines {
		if e := parsePostfixLog(l); e != nil {
			s.add(e)
		}
	}

	d := s.getRecipientDrilldown("sysdig.com", 0)
	if d.Domain != "sysdig.com" {
		t.Errorf("Domain = %q, want sysdig.com", d.Domain)
	}
	if len(d.Recipients) != 2 {
		t.Fatalf("len(Recipients) = %d, want 2", len(d.Recipients))
	}
	// sowji should be first (most failures)
	if d.Recipients[0].Address != "sowji@sysdig.com" {
		t.Errorf("first recipient = %q, want sowji@sysdig.com", d.Recipients[0].Address)
	}
	if d.Recipients[0].Bounced != 3 {
		t.Errorf("sowji bounced = %d, want 3", d.Recipients[0].Bounced)
	}
	if d.Recipients[1].Address != "other@sysdig.com" {
		t.Errorf("second recipient = %q, want other@sysdig.com", d.Recipients[1].Address)
	}
	if d.Recipients[1].Sent != 1 {
		t.Errorf("other sent = %d, want 1", d.Recipients[1].Sent)
	}
	if d.Bounced != 3 || d.Sent != 1 {
		t.Errorf("domain totals: bounced=%d sent=%d, want bounced=3 sent=1", d.Bounced, d.Sent)
	}
}

func TestGetQueueAgeDistribution(t *testing.T) {
	s := newLogStore()

	lines := []string{
		// fast delivery <10s
		`Nov 03 00:00:04 mail postfix/smtp[1]: AAA000000A: to=<a@b.com>, relay=mx[1.2.3.4]:25, delay=2, delays=0.1/0/0.5/1.4, dsn=2.0.0, status=sent (250 ok)`,
		// slow >1h
		`Nov 03 00:00:04 mail postfix/smtp[1]: BBB000000B: to=<c@d.com>, relay=mx[1.2.3.4]:25, delay=7200, delays=7190/5/2/3, dsn=4.7.500, status=deferred (busy)`,
	}
	for _, l := range lines {
		if e := parsePostfixLog(l); e != nil {
			s.add(e)
		}
	}

	dist := s.getQueueAgeDistribution(0)
	if dist.Total != 2 {
		t.Errorf("Total = %d, want 2", dist.Total)
	}
	// First bucket (<10s) should have count=1
	if dist.Buckets[0].Count != 1 {
		t.Errorf("bucket[0] (<10s) count = %d, want 1", dist.Buckets[0].Count)
	}
	// 7200s = 2h, falls in 1-6h bucket (index 6)
	if dist.Buckets[6].Count != 1 {
		t.Errorf("bucket[6] (1-6h) count = %d, want 1", dist.Buckets[6].Count)
	}
	if dist.MaxSecs != 7200 {
		t.Errorf("MaxSecs = %v, want 7200", dist.MaxSecs)
	}
}
