package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ──────────────────────────────────────────────
// Postfix log parser
// ──────────────────────────────────────────────

var logPattern = regexp.MustCompile(`^(\w+\s+\d+\s+\d+:\d+:\d+)\s+(\S+)\s+postfix\/(.+)\[(\d+)\]:\s+(.+)$`)
var queueIDPattern = regexp.MustCompile(`([A-F0-9]{10,})`)
var fromPattern = regexp.MustCompile(`from=<([^>]*)>`)
var toPattern = regexp.MustCompile(`to=<([^>]*)>`)
var relayPattern = regexp.MustCompile(`relay=([^\s,\[]+)`)
var dsnPattern = regexp.MustCompile(`dsn=(\d+\.\d+\.\d+)`)
var smtpMsgPattern = regexp.MustCompile(`\((.+)\)\s*$`)
var delayPattern = regexp.MustCompile(`delay=([\d.]+)`)
var delaysPattern = regexp.MustCompile(`delays=([\d.]+)\/([\d.]+)\/([\d.]+)\/([\d.]+)`)
var sizePattern = regexp.MustCompile(`size=(\d+)`)
var nrcptPattern = regexp.MustCompile(`nrcpt=(\d+)`)

// RCPT-stage policy rejects from smtpd (plain and chrooted service names like
// mail-proxy/smtpd). Queue ID slot: NOQUEUE when nothing was queued; a real
// queue ID when warn_if_reject accepted the mail after logging the warning.
//   EDE018000B: reject_warning: RCPT from name[ip]: 450 4.1.2 <rcpt>: Reason; from=<s> to=<r> proto=...
//   NOQUEUE: reject:         RCPT from name[ip]: 554 5.7.1 <rcpt>: Reason; ...
// reject_warning = warn_if_reject downgrade (mail accepted, would-be-rejected);
// reject = enforced RCPT-stage rejection (bad_rcpt table, enforced DNS check, …).
var rcptRejectPattern = regexp.MustCompile(`^(NOQUEUE|[A-F0-9]+): (reject_warning|reject): RCPT from ([^\[:]+)(?:\[([^\]]*)\])?: (\d{3} \d\.\d\.\d+) <([^>]*)>: ([^;]+)`)

// parsePostfixLog parses a single log line. For multi-line files spanning year boundaries,
// use parsePostfixLogWithState which tracks month regression (Dec→Jan) to detect year crossings.
func parsePostfixLog(rawLog string) *ParsedLogEntry {
	return parsePostfixLogWithState(rawLog, nil, nil)
}

// parsePostfixLogWithState parses a log line with optional state tracking for year boundaries.
// prevMonth: pointer to previous line's month (1-12), updated on success.
// yearOffset: pointer to cumulative year offset from base year, updated when Dec→Jan crossing detected.
// Base year defaults to current UTC year; caller can pre-set yearOffset if file mtime suggests a different base.
func parsePostfixLogWithState(rawLog string, prevMonth *int, yearOffset *int) *ParsedLogEntry {
	// RCPT-stage policy lines from smtpd: warn_if_reject downgrades and hard
	// NOQUEUE rejects. Everything else from smtpd is ignored — admitting all
	// smtpd traffic would flood the store with connect/disconnect noise.
	// "smtpd[" (not "postfix/smtpd[") also matches chrooted service names like
	// postfix/mail-proxy/smtpd — the relay's dedicated listener for the proxy.
	isSmtpdPolicyLine := strings.Contains(rawLog, "smtpd[") &&
		(strings.Contains(rawLog, "reject_warning: RCPT from") ||
			strings.Contains(rawLog, "reject: RCPT from"))
	if !isSmtpdPolicyLine &&
		!strings.Contains(rawLog, "postfix/qmgr[") &&
		!strings.Contains(rawLog, "postfix/smtp[") &&
		!strings.Contains(rawLog, "postfix/error[") {
		return nil
	}

	match := logPattern.FindStringSubmatch(rawLog)
	if match == nil {
		return nil
	}

	timestampStr := match[1]
	service := match[3]
	message := match[5]

	baseYear := time.Now().UTC().Year()
	if yearOffset != nil {
		baseYear += *yearOffset
	}
	ts, err := time.Parse("Jan  2 15:04:05 UTC 2006", fmt.Sprintf("%s UTC %d", timestampStr, baseYear))
	if err != nil {
		ts, err = time.Parse("Jan 2 15:04:05 UTC 2006", fmt.Sprintf("%s UTC %d", timestampStr, baseYear))
		if err != nil {
			return nil
		}
	}

	// Detect Dec→Jan month regression to handle year boundary crossings without relying on wall clock.
	// If we have previous month and current month is January (1) while previous was December (12),
	// we've crossed into the next year.
	thisMonth := int(ts.Month())
	if prevMonth != nil && *prevMonth == 12 && thisMonth == 1 {
		if yearOffset != nil {
			*yearOffset++
		}
		ts = ts.AddDate(1, 0, 0)
	}
	if prevMonth != nil {
		*prevMonth = thisMonth
	}

	// Fallback: if still in future by >24h (e.g., first line of old log parsed with current year),
	// roll back one year. This handles the case where state tracking isn't used or first line is Dec.
	if time.Until(ts) > 24*time.Hour {
		ts = ts.AddDate(-1, 0, 0)
		if prevMonth != nil {
			*prevMonth = int(ts.Month())
		}
		if yearOffset != nil {
			*yearOffset--
		}
	}

	msgLower := strings.ToLower(message)

	// Postfix prefixes its own diagnostics with the severity keyword as the first
	// word of the message (e.g. "warning: ...", "fatal: ..."). Using HasPrefix
	// avoids false positives from DSN text like "Name service error" buried in
	// a status=bounced line.
	isDeliveryLine := strings.Contains(msgLower, "status=")
	isError := !isDeliveryLine && (strings.HasPrefix(msgLower, "error:") || strings.HasPrefix(msgLower, "fatal:") || strings.HasPrefix(msgLower, "panic:"))
	isWarning := !isDeliveryLine && strings.HasPrefix(msgLower, "warning:")

	entry := &ParsedLogEntry{
		ID:        fmt.Sprintf("%d-%s", ts.UnixMilli(), rawLog),
		Timestamp: ts.UnixMilli(),
		Service:   service,
		IsError:   isError,
		IsWarning: isWarning,
	}

	if isError || isWarning {
		entry.RawLog = rawLog
	}

	// RCPT-stage policy events (smtpd only): parse and return early — these
	// lines never carry delivery fields (status/delay/size), and we must not
	// run the generic delivery-line extraction on them.
	// smtpd policy lines can run under chrooted service names (mail-proxy/smtpd).
	if strings.HasSuffix(service, "smtpd") {
		if m := rcptRejectPattern.FindStringSubmatch(message); m != nil {
			entry.Status = "warned"
			if m[2] == "reject" {
				entry.Status = "rejected"
			}
			if m[1] != "NOQUEUE" {
				entry.QueueID = m[1]
			}
			entry.Relay = m[3]
			if m[4] != "" {
				entry.Relay = m[3] + "[" + m[4] + "]"
			}
			entry.DSN = m[5]
			entry.To = m[6]
			entry.SMTPMsg = strings.TrimSpace(m[7])
			entry.RawLog = rawLog
			if fm := fromPattern.FindStringSubmatch(message); fm != nil {
				if fm[1] == "" {
					entry.From = "<>"
				} else {
					entry.From = fm[1]
				}
			}
			return entry
		}
		return nil // smtpd line that looked like policy but isn't parseable
	}

	if m := queueIDPattern.FindStringSubmatch(message); m != nil {
		entry.QueueID = m[1]
	}
	if m := fromPattern.FindStringSubmatch(message); m != nil {
		if m[1] == "" {
			entry.From = "<>" // empty sender = NDR/DSN notification
		} else {
			entry.From = m[1]
		}
	}
	if m := toPattern.FindStringSubmatch(message); m != nil {
		entry.To = m[1]
	}
	if m := sizePattern.FindStringSubmatch(message); m != nil {
		entry.Size, _ = strconv.ParseInt(m[1], 10, 64)
	}
	if m := nrcptPattern.FindStringSubmatch(message); m != nil {
		n, _ := strconv.Atoi(m[1])
		entry.Nrcpt = n
	}
	if m := relayPattern.FindStringSubmatch(message); m != nil && m[1] != "none" {
		entry.Relay = m[1]
	}
	if m := dsnPattern.FindStringSubmatch(message); m != nil {
		entry.DSN = m[1]
	}
	if m := smtpMsgPattern.FindStringSubmatch(message); m != nil {
		msg := m[1]
		// Strip Microsoft trace suffix: " [HOSTNAME timestamp traceID]" — identified by
		// the pattern " [WORD.WORD" (hostname-like, not a bare IP bracket mid-sentence)
		if idx := strings.Index(msg, " ["); idx > 0 {
			tail := msg[idx+2:]
			// only strip if it looks like a hostname (contains a dot before any space)
			dot := strings.Index(tail, ".")
			space := strings.Index(tail, " ")
			if dot >= 0 && (space < 0 || dot < space) {
				msg = strings.TrimSpace(msg[:idx])
			}
		}
		if idx := strings.LastIndex(msg, " (in reply to"); idx > 0 {
			msg = strings.TrimSpace(msg[:idx])
		}
		entry.SMTPMsg = msg
	}

	switch {
	case strings.Contains(msgLower, "status=sent"):
		entry.Status = "sent"
	case strings.Contains(msgLower, "status=deferred"):
		entry.Status = "deferred"
		entry.RawLog = rawLog // need full line for reason extraction
	case strings.Contains(msgLower, "status=bounced"):
		entry.Status = "bounced"
		entry.RawLog = rawLog // need full line for reason extraction
	case (strings.Contains(msgLower, "status=expired") || strings.Contains(msgLower, "returned to sender")) &&
		!strings.Contains(service, "qmgr"):
		entry.Status = "bounced"
	case strings.Contains(msgLower, "removed"):
		entry.Status = "removed"
	case strings.Contains(msgLower, "queue active"):
		entry.Status = "active"
	}

	// Precompute reason + taxonomy bucket once — queries re-use these instead of
	// re-running regex rules over RawLog on every scan.
	switch entry.Status {
	case "deferred":
		entry.DeferReason = extractDeferReason(entry.RawLog)
		entry.DeferBucket = classifyBucket(entry.DeferReason, "deferred", entry.DSN)
	case "bounced":
		entry.BounceReason = extractBounceReason(entry.RawLog)
		entry.BounceBucket = classifyBucket(entry.BounceReason, "bounced", entry.DSN)
	}

	if m := delayPattern.FindStringSubmatch(message); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			entry.Delay = v
		}
	}
	if m := delaysPattern.FindStringSubmatch(message); m != nil {
		bq, _ := strconv.ParseFloat(m[1], 64)
		iq, _ := strconv.ParseFloat(m[2], 64)
		cs, _ := strconv.ParseFloat(m[3], 64)
		tx, _ := strconv.ParseFloat(m[4], 64)
		entry.Delays = &DelayBreakdown{
			BeforeQmgr:   bq,
			InQmgr:       iq,
			ConnSetup:    cs,
			Transmission: tx,
		}
	}

	return entry
}

// extractDomain returns the domain part of an email address.
func extractDomain(email string) string {
	if idx := strings.LastIndex(email, "@"); idx >= 0 {
		return email[idx+1:]
	}
	return "unknown"
}

// ── Reason classification ─────────────────────────────────────────────────────

// reasonRule maps a set of substrings to a category label.
// any:  at least one of anyOf must be present in the lowercased line.
// all:  all of allOf must also be present (used for AND conditions).
type reasonRule struct {
	reason string
	anyOf  []string // OR — at least one must match
	allOf  []string // AND — all must match (optional)
}

// containsAny returns true if low contains at least one of the given substrings.
func containsAny(low string, subs []string) bool {
	for _, s := range subs {
		if strings.Contains(low, s) {
			return true
		}
	}
	return false
}

// containsAll returns true if low contains every one of the given substrings.
func containsAll(low string, subs []string) bool {
	for _, s := range subs {
		if !strings.Contains(low, s) {
			return false
		}
	}
	return true
}

// matchReason returns the first rule whose pattern matches low, or "" if none do.
func matchReason(low string, rules []reasonRule) string {
	for _, r := range rules {
		if containsAny(low, r.anyOf) && (len(r.allOf) == 0 || containsAll(low, r.allOf)) {
			return r.reason
		}
	}
	return ""
}

var deferRules = []reasonRule{
	{reason: "Network unreachable (IPv6)", anyOf: []string{"network is unreachable", "delivery temporarily suspended"}},
	{reason: "DNS failure", anyOf: []string{"host not found", "name service error", "dns"}},
	{reason: "Connection refused", anyOf: []string{"connection refused"}},
	{reason: "Connection refused", anyOf: []string{"refused"}, allOf: []string{"connect to"}},
	{reason: "Lost connection", anyOf: []string{"lost connection", "connection reset"}},
	{reason: "Timeout", anyOf: []string{"connection timed out", "timed out"}},
	{reason: "Rate limited", anyOf: []string{"too many connections", "rate limit", "421"}},
	{reason: "Server busy (M365)", anyOf: []string{"server busy", "try again later"}},
	{reason: "Greylisted", anyOf: []string{"greylisted", "greylist"}},
	{reason: "Unauthenticated", anyOf: []string{"unauthenticated", "4.4.4"}},
	{reason: "Mailbox full", anyOf: []string{"out of storage", "over quota", "4.2.2", "mailbox full"}},
	{reason: "Remote server error", anyOf: []string{"internal resource", "temporarily unavailable"}},
	{reason: "TLS error", anyOf: []string{"tls", "certificate"}},
	{reason: "Policy block", anyOf: []string{"policy", "blocked"}},
	{reason: "IP/domain blocked", anyOf: []string{"blocked", "blocklist", "blacklist", "spam"}},
	{reason: "Unknown recipient", anyOf: []string{"4.1.", "no such user", "unknown user"}},
	// DSN code families — vendor-independent structural fallback (RFC 3463).
	// Placed last so keyword rules win when both are present.
	{reason: "Mailbox full", anyOf: []string{"4.2."}},
	{reason: "Remote server error", anyOf: []string{"4.4."}},
	{reason: "Policy block", anyOf: []string{"4.7."}},
}

var bounceRules = []reasonRule{
	{reason: "Unknown user (5.1.1)", anyOf: []string{"5.1.1", "user unknown", "no such user"}},
	{reason: "Mailbox not found (5.0.0)", anyOf: []string{"mailbox not found", "552", "requested mail action aborted"}},
	{reason: "Inactive account (5.2.1)", anyOf: []string{"5.2.1", "inactive", "disabled"}},
	{reason: "Mailbox full (5.2.2)", anyOf: []string{"5.2.2", "over quota", "mailbox full"}},
	{reason: "Address rejected (5.4.1)", anyOf: []string{"5.4.1", "access denied", "mailbox unavailable"}},
	{reason: "Address rejected (5.1.0)", anyOf: []string{"5.1.0", "address rejected"}},
	{reason: "Policy rejection (5.7.1)", anyOf: []string{"5.7.1"}, allOf: []string{"reject"}},
	{reason: "Policy rejection (5.7.1)", anyOf: []string{"5.7.1"}, allOf: []string{"policy"}},
	{reason: "IP/domain blocked", anyOf: []string{"blocked", "blacklist", "blocklist", "spam"}},
	{reason: "DNS failure", anyOf: []string{"host not found", "name service error", "name or service not known"}},
	{reason: "Bad destination (5.1.2)", anyOf: []string{"5.1.2", "bad destination"}},
	{reason: "Auth/policy failure (5.7.x)", anyOf: []string{"5.7.", "authentication", "dmarc", "spf", "dkim"}},
	{reason: "Service unavailable (5.3.x)", anyOf: []string{"5.3.", "service unavailable"}},
}

// extractDeferReason returns a short categorised reason from a deferred log line.
func extractDeferReason(raw string) string {
	if raw == "" {
		return ""
	}
	if r := matchReason(strings.ToLower(raw), deferRules); r != "" {
		return r
	}
	return "Other"
}

// extractBounceReason categorises a bounced log line by 5xx DSN code and keywords.
func extractBounceReason(raw string) string {
	if raw == "" {
		return ""
	}
	if r := matchReason(strings.ToLower(raw), bounceRules); r != "" {
		return r
	}
	return "Other"
}

// ── Bounce taxonomy — the "what kind of problem is this" layer above fine reasons ──

// reasonBuckets maps an extracted defer/bounce reason to its coarse taxonomy bucket.
var reasonBuckets = map[string]string{
	// deferred + shared
	"Network unreachable (IPv6)":  "Connection / network",
	"DNS failure":                 "Connection / network",
	"Connection refused":          "Connection / network",
	"Lost connection":             "Connection / network",
	"Timeout":                     "Connection / network",
	"TLS error":                   "Connection / network",
	"Rate limited":                "Soft / temporary",
	"Server busy (M365)":          "Soft / temporary",
	"Greylisted":                  "Soft / temporary",
	"Unauthenticated":             "Soft / temporary",
	"Mailbox full":                "Soft / temporary",
	"Remote server error":         "Soft / temporary",
	"Unknown recipient":           "Soft / temporary", // deferred on 4.1.x — still retrying
	"Policy block":                "Block / reputation",
	"IP/domain blocked":           "Block / reputation",
	// bounced
	"Unknown user (5.1.1)":        "Hard bounce",
	"Mailbox not found (5.0.0)":   "Hard bounce",
	"Inactive account (5.2.1)":    "Hard bounce",
	"Mailbox full (5.2.2)":        "Hard bounce",
	"Address rejected (5.4.1)":    "Hard bounce",
	"Address rejected (5.1.0)":    "Hard bounce",
	"Policy rejection (5.7.1)":    "Block / reputation",
	"Bad destination (5.1.2)":     "Hard bounce",
	"Auth/policy failure (5.7.x)": "Block / reputation",
	"Service unavailable (5.3.x)": "Hard bounce",
}

// classifyBucket maps an extracted reason to a taxonomy bucket, falling back
// to the DSN family (5.x.x permanent, 4.x.x temporary) and finally "Other".
func classifyBucket(reason, status, dsn string) string {
	if reason != "" && reason != "Other" {
		if b, ok := reasonBuckets[reason]; ok {
			return b
		}
		if status == "bounced" {
			return "Hard bounce"
		}
		if status == "deferred" {
			return "Soft / temporary"
		}
	}
	if strings.HasPrefix(dsn, "5.") {
		return "Hard bounce"
	}
	if strings.HasPrefix(dsn, "4.") {
		return "Soft / temporary"
	}
	return "Other"
}
