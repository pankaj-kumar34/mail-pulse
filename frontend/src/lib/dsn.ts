// Plain-English explanations for common Postfix DSN codes (RFC 3463 families).
// Shown as tooltips on DSN badges throughout the app.

const DSN_TEXT: Record<string, string> = {
  "2.0.0": "Delivered — success",
  "4.2.1": "Mailbox busy — recipient over load, retry later",
  "4.2.2": "Mailbox full — recipient over quota, retry later",
  "4.3.1": "Mail system full — remote out of resources",
  "4.3.2": "System temporarily refuses to accept mail",
  "4.4.1": "No answer from remote host — connection timeout",
  "4.4.2": "Bad connection — dropped mid-transfer",
  "4.4.4": "Unable to route — often unauthenticated relay config",
  "4.7.0": "Policy throttle — security/anti-spam, try later",
  "4.7.1": "Blocked by policy — reputation/anti-spam, may persist",
  "5.0.0": "Permanent failure — remote refused with no detail",
  "5.1.0": "Address rejected — recipient refused by remote",
  "5.1.1": "User unknown — mailbox doesn't exist, don't retry",
  "5.1.2": "Bad destination — domain has no valid route",
  "5.2.1": "Mailbox disabled / inactive",
  "5.2.2": "Mailbox full — bounced after retries exhausted",
  "5.3.0": "Mail system problem — permanent remote failure",
  "5.3.4": "Message too big for remote system",
  "5.4.1": "Access denied — relay/recipient refused",
  "5.7.1": "Rejected by policy — blocked/spam/auth failure",
};

export function dsnExplain(dsn: string): string {
  if (!dsn) return "";
  if (DSN_TEXT[dsn]) return `${dsn} — ${DSN_TEXT[dsn]}`;
  if (dsn.startsWith("5.")) return `${dsn} — permanent failure (5.x.x)`;
  if (dsn.startsWith("4.")) return `${dsn} — temporary failure, will retry (4.x.x)`;
  return dsn;
}
