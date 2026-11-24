package main

import (
	"strings"
	"testing"
)

// ──────────────────────────────────────────────
// parsePostfixLog
// ──────────────────────────────────────────────

func TestParsePostfixLog_Sent(t *testing.T) {
	raw := `Nov 03 00:00:04 mail postfix/smtp[129270]: 2EA1F80020: to=<jamie.calhoun415@example.com>, relay=mx0b-001b2d05.pphosted.com[205.220.172.48]:25, delay=1.9, delays=0.21/0.49/0.35/0.83, dsn=2.0.0, status=sent (250 2.0.0 Message accepted)`
	e := parsePostfixLog(raw)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if e.Status != "sent" {
		t.Errorf("Status = %q, want %q", e.Status, "sent")
	}
	if e.To != "jamie.calhoun415@example.com" {
		t.Errorf("To = %q, want jamie.calhoun415@example.com", e.To)
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
	raw := `Nov 03 00:00:03 mail postfix/smtp[129254]: D9D5E80236: to=<dana.kowalski72@example.org>, relay=example-org.mail.protection.outlook.com[52.101.60.182]:25, delay=8713, delays=8711/0.32/1.1/0.32, dsn=4.7.500, status=deferred (host said: 451 4.7.500 Server busy)`
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
	raw := `Nov 03 00:00:13 mail postfix/smtp[129263]: 4E84780258: to=<riley.merchant83+reports@example.net>, relay=aspmx.l.google.com[172.253.156.26]:25, delay=4.5, delays=0.18/0/4.2/0.18, dsn=5.2.1, status=bounced (host said: 550-5.2.1 The email account that you tried to reach is inactive)`
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
	raw := `Nov 03 00:49:30 mail postfix/smtp[129619]: D6AAE80590: to=<casey.donnelly29@example.org>, relay=none, delay=0.07, delays=0.02/0/0.05/0, dsn=5.4.4, status=bounced (Host or domain name not found. Name service error for name=example.org type=AAAA: Host not found)`
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
	raw := `Nov 03 00:00:02 mail postfix/qmgr[129239]: 3FFD980237: from=<no-reply@insights.example.com>, size=1706, nrcpt=1 (queue active)`
	e := parsePostfixLog(raw)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if e.Status != "active" {
		t.Errorf("Status = %q, want active", e.Status)
	}
	if e.From != "no-reply@insights.example.com" {
		t.Errorf("From = %q, want no-reply@insights.example.com", e.From)
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
	raw := `Nov 03 00:01:02 mail postfix/error[129292]: B2D2480085: to=<no-reply@insights.example.com>, relay=none, delay=56132, delays=56071/60/0/0, dsn=4.4.1, status=deferred (delivery temporarily suspended: connect to insights.ibm.com[2606:4700::6812:1ff7]:25: Network is unreachable)`
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
		{"host mail1.nrwbank.de[213.86.233.144] refused to talk to me: 550 5.5.0 67.18.99.231 is blocklisted by FortiGuard. This email from IP  has been rejected. The email message was detected as spam.", "IP/domain blocked"},
		{"450 4.7.1 access denied for unknown reasons", "Policy block"},
		{"450 4.2.0 requested action aborted", "Mailbox full"},
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
	normal := parsePostfixLog(`Nov 03 00:00:04 mail postfix/smtp[129270]: 2EA1F80020: to=<taylor.brooks58@example.com>, relay=mx.ibm.com[1.2.3.4]:25, delay=1.9, delays=0.21/0.49/0.35/0.83, dsn=2.0.0, status=sent (250 ok)`)

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

	raw := `Nov 03 00:00:04 mail postfix/smtp[129270]: 2EA1F80020: to=<taylor.brooks58@example.com>, relay=mx.ibm.com[1.2.3.4]:25, delay=1.9, delays=0.21/0.49/0.35/0.83, dsn=2.0.0, status=sent (250 ok)`
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
// extractBlockEvent — IP reputation patterns
// ──────────────────────────────────────────────

func TestExtractBlockEvent(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantIP  string
		wantSvc string
		wantOK  bool
	}{
		{
			name:    "FortiGuard refused to talk (deferred)",
			raw:     `Nov 16 06:13:15 mail postfix/smtp[28988]: 3DE512001B: to=<th@nrwk.de>, relay=mail1.nrwbank.de[213.86.233.144]:25, delay=2, delays=0.02/0/2/0, dsn=4.5.0, status=deferred (host mail1.nrwbank.de[213.86.233.144] refused to talk to me: 550 5.5.0 67.18.99.231 is blocklisted by FortiGuard. This email from IP  has been rejected. The email message was detected as spam.)`,
			wantIP:  "67.18.99.231",
			wantSvc: "FortiGuard",
			wantOK:  true,
		},
		{
			name:    "TrendMicro ERS",
			raw:     `Nov 16 00:00:01 mail postfix/smtp[1]: AAA0000001: to=<a@b.com>, relay=x[1.2.3.4]:25, delay=1, delays=0/0/0/1, dsn=5.0.0, status=bounced (host mail.example.com[1.2.3.4] said: 553 Mail from <67.18.99.231> was refused due to the sender IP found in ERS-DUL (in reply to MAIL FROM command))`,
			wantIP:  "67.18.99.231",
			wantSvc: "TrendMicro ERS-DUL",
			wantOK:  true,
		},
		{
			name:    "Client host blocked using",
			raw:     `Nov 16 00:00:01 mail postfix/smtpd[1]: NOQUEUE: reject: RCPT from unknown[1.2.3.4]: 554 5.7.1 Service unavailable; Client host [67.18.99.231] blocked using TrendMicro-dul; from=<x@y.com> to=<a@b.com>`,
			wantIP:  "67.18.99.231",
			wantSvc: "TrendMicro Email Reputation",
			wantOK:  true,
		},
		{
			name:    "no block event",
			raw:     `Nov 16 00:00:01 mail postfix/smtp[1]: AAA0000002: to=<a@b.com>, relay=x[1.2.3.4]:25, delay=1, delays=0/0/0/1, dsn=4.2.1, status=deferred (host said: 450 4.2.1 mailbox busy)`,
			wantOK:  false,
		},
		{
			// Novel format #1 — Spamhaus "listed by" phrasing, no dedicated regex.
			name:    "generic: Spamhaus listed-by (future format)",
			raw:     `Nov 16 07:00:00 mail postfix/smtp[1]: AAA0000003: to=<a@b.com>, relay=mx.example.com[1.2.3.4]:25, delay=1, delays=0/0/0/1, dsn=5.7.1, status=bounced (host mx.example.com[1.2.3.4] said: 550 5.7.1 67.18.99.231 listed by zen.spamhaus.org)`,
			wantIP:  "67.18.99.231",
			wantSvc: "Spamhaus",
			wantOK:  true,
		},
		{
			// Novel format #2 — vendor we've never seen, no keyword match in vocab vendor list.
			name:    "generic: unknown vendor blocked-by (future format)",
			raw:     `Nov 16 08:00:00 mail postfix/smtp[1]: AAA0000004: to=<a@b.com>, relay=mx.other.com[9.8.7.6]:25, delay=1, delays=0/0/0/1, dsn=5.7.1, status=bounced (host mx.other.com[9.8.7.6] said: 554 5.7.1 connections not accepted from 67.18.99.231: blocked by Acme AntiSpam Shield)`,
			wantIP:  "67.18.99.231",
			wantSvc: "Acme",
			wantOK:  true,
		},
		{
			// "blocked by" WITHOUT a bare IP must not fire.
			name:    "negative: blocked by policy, no bare IP",
			raw:     `Nov 16 09:00:00 mail postfix/smtp[1]: AAA0000005: to=<a@b.com>, relay=mx.example.com[1.2.3.4]:25, delay=1, delays=0/0/0/1, dsn=5.7.1, status=bounced (host mx.example.com[1.2.3.4] said: 550 5.7.1 message blocked by policy)`,
			wantOK:  false,
		},
		{
			// Private/RFC1918 bare IP is our own infra — not a blocklist event.
			name:    "negative: only private bare IP",
			raw:     `Nov 16 10:00:00 mail postfix/smtp[1]: AAA0000006: to=<a@b.com>, relay=mx.example.com[1.2.3.4]:25, delay=1, delays=0/0/0/1, dsn=5.7.1, status=bounced (host mx.example.com[1.2.3.4] said: 550 5.7.1 client 10.0.0.5 blocked by spamhaus)`,
			wantOK:  false,
		},
	}
	for _, c := range cases {
		ip, svc, ok := extractBlockEvent(c.raw)
		if ok != c.wantOK {
			t.Errorf("%s: ok = %v, want %v", c.name, ok, c.wantOK)
			continue
		}
		if ok && ip != c.wantIP {
			t.Errorf("%s: ip = %q, want %q", c.name, ip, c.wantIP)
		}
		if ok && svc != c.wantSvc {
			t.Errorf("%s: svc = %q, want %q", c.name, svc, c.wantSvc)
		}
	}
}

// End-to-end: a FortiGuard deferred line must produce an IP reputation stat.
func TestIPReputation_FortiGuardDeferred(t *testing.T) {
	s := newLogStore()
	raw := `Nov 16 06:13:15 mail postfix/smtp[28988]: 3DE512001B: to=<th@nrwk.de>, relay=mail1.nrwbank.de[213.86.233.144]:25, delay=2, delays=0.02/0/2/0, dsn=4.5.0, status=deferred (host mail1.nrwbank.de[213.86.233.144] refused to talk to me: 550 5.5.0 67.18.99.231 is blocklisted by FortiGuard. This email from IP  has been rejected. The email message was detected as spam.)`
	e := parsePostfixLog(raw)
	if e == nil {
		t.Fatal("must parse")
	}
	s.add(e)

	rep := s.getIPReputationReport(0)
	if len(rep.Stats) != 1 {
		t.Fatalf("got %d stats, want 1", len(rep.Stats))
	}
	st := rep.Stats[0]
	if st.OurIP != "67.18.99.231" {
		t.Errorf("OurIP = %q, want 67.18.99.231", st.OurIP)
	}
	if st.Service != "FortiGuard" {
		t.Errorf("Service = %q, want FortiGuard", st.Service)
	}
	if st.Count != 1 {
		t.Errorf("Count = %d, want 1", st.Count)
	}
	if len(st.Domains) != 1 || st.Domains[0] != "nrwk.de" {
		t.Errorf("Domains = %v, want [nrwk.de]", st.Domains)
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
		// 3 bounces to sam.whitfield09@example.net
		`Nov 03 00:00:13 mail postfix/smtp[1]: AAA000000A: to=<sam.whitfield09@example.net>, relay=aspmx.l.google.com[1.2.3.4]:25, delay=4.5, delays=0.18/0/4.2/0.18, dsn=5.2.1, status=bounced (550 5.2.1 inactive)`,
		`Nov 03 00:00:19 mail postfix/smtp[1]: BBB000000B: to=<sam.whitfield09@example.net>, relay=aspmx.l.google.com[1.2.3.4]:25, delay=4.5, delays=0.16/0/4.1/0.21, dsn=5.2.1, status=bounced (550 5.2.1 inactive)`,
		`Nov 03 00:00:24 mail postfix/smtp[1]: CCC000000C: to=<sam.whitfield09@example.net>, relay=aspmx.l.google.com[1.2.3.4]:25, delay=4.4, delays=0.17/0/4.1/0.19, dsn=5.2.1, status=bounced (550 5.2.1 inactive)`,
		// 1 sent to dana.lopes77@example.net
		`Nov 03 00:00:30 mail postfix/smtp[1]: DDD000000D: to=<dana.lopes77@example.net>, relay=aspmx.l.google.com[1.2.3.4]:25, delay=1.0, delays=0.1/0/0.5/0.4, dsn=2.0.0, status=sent (250 ok)`,
	}
	for _, l := range lines {
		if e := parsePostfixLog(l); e != nil {
			s.add(e)
		}
	}

	d := s.getRecipientDrilldown("example.net", 0)
	if d.Domain != "example.net" {
		t.Errorf("Domain = %q, want example.net", d.Domain)
	}
	if len(d.Recipients) != 2 {
		t.Fatalf("len(Recipients) = %d, want 2", len(d.Recipients))
	}
	// sam should be first (most failures)
	if d.Recipients[0].Address != "sam.whitfield09@example.net" {
		t.Errorf("first recipient = %q, want sam.whitfield09@example.net", d.Recipients[0].Address)
	}
	if d.Recipients[0].Bounced != 3 {
		t.Errorf("sam bounced = %d, want 3", d.Recipients[0].Bounced)
	}
	if d.Recipients[1].Address != "dana.lopes77@example.net" {
		t.Errorf("second recipient = %q, want dana.lopes77@example.net", d.Recipients[1].Address)
	}
	if d.Recipients[1].Sent != 1 {
		t.Errorf("dana sent = %d, want 1", d.Recipients[1].Sent)
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

// ──────────────────────────────────────────────
// RCPT-stage policy lines: warn_if_reject + NOQUEUE reject
// ──────────────────────────────────────────────

func TestParseRejectWarning(t *testing.T) {
	line := `Nov 20 10:15:32 relay01 postfix/smtpd[4711]: NOQUEUE: reject_warning: RCPT from relay02[10.20.30.40]: 554 5.7.1 <qa-fixture@dps.openpages.local>: Recipient address rejected: Domain not found; from=<test@acme.com> to=<qa-fixture@dps.openpages.local> proto=ESMTP helo=<qa-pod>`
	e := parsePostfixLog(line)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if e.Status != "warned" {
		t.Errorf("Status = %q, want %q", e.Status, "warned")
	}
	if e.To != "qa-fixture@dps.openpages.local" {
		t.Errorf("To = %q", e.To)
	}
	if e.From != "test@acme.com" {
		t.Errorf("From = %q", e.From)
	}
	if e.DSN != "554 5.7.1" {
		t.Errorf("DSN = %q", e.DSN)
	}
	if e.SMTPMsg != "Recipient address rejected: Domain not found" {
		t.Errorf("SMTPMsg = %q", e.SMTPMsg)
	}
	if e.Relay != "relay02[10.20.30.40]" {
		t.Errorf("Relay = %q", e.Relay)
	}
	if e.QueueID != "" {
		t.Errorf("QueueID = %q, want empty (NOQUEUE)", e.QueueID)
	}
	if e.IsWarning || e.IsError {
		t.Error("warned entries must not pollute error/warning diagnostics")
	}
}

func TestParseRejectWarningQueueIDChrooted(t *testing.T) {
	// Live line from the test relay 2026-11-24: warn_if_reject ACCEPTED the mail,
	// so the warning carries a real queue ID (not NOQUEUE) and a chrooted service
	// name (mail-proxy/smtpd) — both previously unparsed (Warn Monitor showed nothing).
	line := `Nov 24 03:34:03 mail postfix/mail-proxy/smtpd[104]: EDE018000B: reject_warning: RCPT from unknown[172.17.1.152]: 450 4.1.2 <test@warn-if-reject-test.xyz>: Recipient address rejected: Domain not found; from=<pankaj@mailx.event-notifications.test.cloud.ibm.com> to=<test@warn-if-reject-test.xyz> proto=ESMTP helo=<mail.event-notifications.test.cloud.ibm.com>`
	e := parsePostfixLog(line)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if e.Status != "warned" {
		t.Errorf("Status = %q, want %q", e.Status, "warned")
	}
	if e.QueueID != "EDE018000B" {
		t.Errorf("QueueID = %q, want %q (accepted-after-warn lines carry a real queue ID)", e.QueueID, "EDE018000B")
	}
	if e.To != "test@warn-if-reject-test.xyz" {
		t.Errorf("To = %q", e.To)
	}
	if e.From != "pankaj@mailx.event-notifications.test.cloud.ibm.com" {
		t.Errorf("From = %q", e.From)
	}
	if e.DSN != "450 4.1.2" {
		t.Errorf("DSN = %q", e.DSN)
	}
	if e.SMTPMsg != "Recipient address rejected: Domain not found" {
		t.Errorf("SMTPMsg = %q", e.SMTPMsg)
	}
	if e.Relay != "unknown[172.17.1.152]" {
		t.Errorf("Relay = %q", e.Relay)
	}
	if e.Service != "mail-proxy/smtpd" {
		t.Errorf("Service = %q", e.Service)
	}
}

func TestParseRcptStageHardReject(t *testing.T) {
	line := `Nov 20 11:02:01 relay01 postfix/smtpd[4812]: NOQUEUE: reject: RCPT from relay02[10.20.30.40]: 554 5.7.1 <test-user@sanfran.net>: Recipient address rejected: recipient domain does not exist (SBL incident 2026-11); from=<x@y.com> to=<test-user@sanfran.net> proto=ESMTP helo=<h1>`
	e := parsePostfixLog(line)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if e.Status != "rejected" {
		t.Errorf("Status = %q, want %q", e.Status, "rejected")
	}
	if e.To != "test-user@sanfran.net" {
		t.Errorf("To = %q", e.To)
	}
	if e.SMTPMsg != "Recipient address rejected: recipient domain does not exist (SBL incident 2026-11)" {
		t.Errorf("SMTPMsg = %q", e.SMTPMsg)
	}
}

func TestNonFqdnWarnNoBrackets(t *testing.T) {
	// Bare hostname (no [ip]) and 504 DSN variant.
	line := `Nov 20 09:00:00 relay01 postfix/smtpd[4900]: NOQUEUE: reject_warning: RCPT from qa-runner: 504 5.5.2 <user@localhost>: Recipient address rejected: need fully-qualified address; from=<a@b.com> to=<user@localhost> proto=SMTP`
	e := parsePostfixLog(line)
	if e == nil {
		t.Fatal("expected entry, got nil")
	}
	if e.Status != "warned" || e.DSN != "504 5.5.2" {
		t.Errorf("Status=%q DSN=%q", e.Status, e.DSN)
	}
	if e.Relay != "qa-runner" {
		t.Errorf("Relay = %q, want bare hostname", e.Relay)
	}
}

func TestSmtpdNonPolicyLinesStillDropped(t *testing.T) {
	for _, line := range []string{
		`Nov 20 10:00:00 relay01 postfix/smtpd[1234]: connect from relay02[10.20.30.40]`,
		`Nov 20 10:00:01 relay01 postfix/smtpd[1234]: disconnect from relay02[10.20.30.40] ehlo=1 mail=1 rcpt=0/1 data=0/1 quit=1 commands=5/6`,
		`Nov 20 10:00:00 relay01 postfix/smtpd[1234]: NOQUEUE: reject: CONNECT from relay02[10.20.30.40]: 554 5.7.1 Client host rejected: access denied; proto=SMTP`,
	} {
		if e := parsePostfixLog(line); e != nil {
			t.Errorf("smtpd non-RCPT line should be dropped: %q → %+v", line, e)
		}
	}
}
