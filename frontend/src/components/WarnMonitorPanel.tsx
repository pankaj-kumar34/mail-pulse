import { useCallback, useEffect, useState } from "react";
import {
  ShieldAlert,
  EyeOff,
  Globe,
  RefreshCw,
  CheckCircle2,
  AlertTriangle,
  HelpCircle,
} from "lucide-react";
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from "recharts";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";
import type { WarnInsights, WarnDomainStat, DomainDNSStatus } from "../../bindings/github.com/pankajkumar/mail-pulse/models";
import { MetricCard } from "./MetricCard";
import { EmptyState } from "./EmptyState";

interface Props {
  sinceHours: number;
  refreshKey?: number;
}

const DNS_PILL: Record<string, { bg: string; fg: string; label: string; hint: string }> = {
  nxdomain: { bg: "#fff1f2", fg: "#e11d48", label: "DEAD", hint: "No MX, no A — enforce would reject it. Garbage." },
  "a-only": { bg: "#fffbeb", fg: "#d97706", label: "A ONLY", hint: "No MX but A record exists — postfix accepts today AND after flip (A fallback)." },
  ok: { bg: "#eff6ff", fg: "#2563eb", label: "LIVE MX", hint: "Has MX now — DNS was transient when warned, or recovered. No flip risk." },
  tempfail: { bg: "#f4f4f5", fg: "#71717a", label: "TEMPFAIL", hint: "Resolver trouble — inconclusive, re-check." },
};

function fmtTime(ms: number) {
  return new Date(ms).toLocaleString([], {
    month: "short", day: "numeric", hour: "2-digit", minute: "2-digit",
  });
}

function fmtHour(ms: number) {
  return new Date(ms).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

export function WarnMonitorPanel({ sinceHours, refreshKey }: Props) {
  const [insights, setInsights] = useState<WarnInsights | null>(null);
  const [loading, setLoading] = useState(true);
  const [dnsStatus, setDnsStatus] = useState<Record<string, DomainDNSStatus>>({});
  const [dnsLoading, setDnsLoading] = useState(false);

  useEffect(() => {
    let alive = true;
    setLoading(true);
    (LogService.GetWarnInsights(sinceHours) as any)
      .then((d: WarnInsights) => { if (alive) setInsights(d); })
      .catch(() => { /* keep previous */ })
      .finally(() => { if (alive) setLoading(false); });
    return () => { alive = false; };
  }, [sinceHours, refreshKey]);

  const runDnsCheck = useCallback(async () => {
    if (!insights?.topWarnDomains?.length) return;
    setDnsLoading(true);
    try {
      const domains = insights.topWarnDomains.map((d) => d.domain);
      const res = (await (LogService.CheckWarnDomains(domains) as any)) as DomainDNSStatus[] | null;
      const next: Record<string, DomainDNSStatus> = { ...dnsStatus };
      for (const r of res ?? []) next[r.domain] = r;
      setDnsStatus(next);
    } finally {
      setDnsLoading(false);
    }
  }, [insights, dnsStatus]);

  if (loading && !insights) {
    return (
      <div className="h-full flex items-center justify-center">
        <RefreshCw size={18} className="animate-spin text-zinc-300" />
      </div>
    );
  }

  if (!insights || (insights.total === 0 && insights.rejectedTotal === 0)) {
    return (
      <EmptyState
        icon="check"
        title="No RCPT-stage policy events in this window"
        subtitle="warn_if_reject downgrades and hard NOQUEUE rejects from postfix/smtpd will appear here. Nothing warned or rejected yet — either the log has none, or it hasn't been re-ingested since the config landed."
      />
    );
  }

  const checked = Object.values(dnsStatus);
  const deadCount = checked.filter((d) => d.status === "nxdomain").length;
  const tempfailCount = checked.filter((d) => d.status === "tempfail").length;
  const aliveCount = checked.filter((d) => d.status === "ok" || d.status === "a-only").length;

  let verdict: { tone: "good" | "warn" | "info"; text: string } | null = null;
  if (checked.length > 0) {
    if (tempfailCount === 0) {
      verdict = {
        tone: "good",
        text: `Checked ${checked.length} domains: ${deadCount} fully dead, ${aliveCount} resolve (and would NOT be rejected by enforce — the check only rejects when MX and A are both absent). No false-reject evidence here; eyeball the names above for anything that looks like a real partner domain, then flip.`,
      };
    } else {
      verdict = {
        tone: "warn",
        text: `${tempfailCount} domain(s) inconclusive (resolver tempfail) — re-check before using this as flip evidence.`,
      };
    }
  }

  const hourly = (insights.hourly ?? []).map((h) => ({
    hour: h.hour,
    Warned: h.warned,
    Rejected: h.rejected,
  }));

  return (
    <div className="h-full overflow-y-auto p-4 space-y-4">
      {/* ── Explainer ─────────────────────────────────────────────── */}
      <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3">
        <div className="flex items-start gap-2.5">
          <ShieldAlert size={16} className="text-amber-600 shrink-0 mt-0.5" />
          <div className="text-xs leading-relaxed text-amber-900">
            <span className="font-semibold">warn_if_reject reject_unknown_recipient_domain</span> is in{" "}
            <span className="font-semibold">monitor mode</span> on the relay: mail to domains with no MX/A record is{" "}
            <span className="font-semibold">accepted and logged</span> instead of rejected. This page is the flip-decision
            evidence: everything here <em>would</em> bounce once enforced. Flip is safe when the warned domains are all
            garbage and nothing looks like a legitimate partner domain. Live DNS check classifies each domain:{" "}
            <span className="font-medium">DEAD</span> = rejected when enforced · <span className="font-medium">A ONLY</span>{" "}
            and <span className="font-medium">LIVE MX</span> = resolve, so enforce won't touch them ·{" "}
            <span className="font-medium">TEMPFAIL</span> = inconclusive.
          </div>
        </div>
      </div>

      {/* ── Metric cards ──────────────────────────────────────────── */}
      <div className="grid grid-cols-4 gap-3">
        <MetricCard title="Warned (accepted)" value={insights.total} icon={EyeOff} color="yellow"
          subtitle="would-be rejects, currently delivered" />
        <MetricCard title="Hard RCPT rejects" value={insights.rejectedTotal} icon={ShieldAlert} color="red"
          subtitle="blocked at RCPT (table etc.)" />
        <MetricCard title="Warned domains" value={insights.uniqueDomains} icon={Globe} color="blue"
          subtitle="unique recipient domains" />
        <MetricCard title="Unique recipients" value={insights.uniqueRecipients} icon={Globe} color="zinc"
          subtitle={`from ${insights.uniqueClients} client(s)`} />
      </div>

      {/* ── Hourly chart ──────────────────────────────────────────── */}
      {hourly.length > 0 && (
        <div className="rounded-xl border border-zinc-100 bg-white p-4">
          <h3 className="text-sm font-semibold text-zinc-800 mb-1">RCPT-stage policy events per hour</h3>
          <p className="text-xs text-zinc-400 mb-3">Warned = accepted under warn_if_reject · Rejected = blocked at RCPT</p>
          <div style={{ width: "100%", height: 180 }}>
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={hourly} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#f4f4f5" />
                <XAxis dataKey="hour" tickFormatter={fmtHour} tick={{ fontSize: 11, fill: "#a1a1aa" }} axisLine={false} tickLine={false} />
                <YAxis tick={{ fontSize: 11, fill: "#a1a1aa" }} width={36} axisLine={false} tickLine={false} allowDecimals={false} />
                <Tooltip
                  contentStyle={{ borderRadius: 8, border: "1px solid #e4e4e7", fontSize: 12 }}
                  labelFormatter={(v) => fmtHour(Number(v))}
                  cursor={{ fill: "#f4f4f5" }}
                />
                <Legend wrapperStyle={{ fontSize: 12, color: "#71717a" }} />
                <Bar dataKey="Warned" stackId="a" fill="#f59e0b" radius={[0, 0, 2, 2]} />
                <Bar dataKey="Rejected" stackId="a" fill="#ef4444" radius={[2, 2, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      )}

      {/* ── Top warned domains ────────────────────────────────────── */}
      <DomainTable
        title="Top warned domains — the flip evidence"
        subtitle="Every RCPT here was accepted but would be rejected once warn_if_reject is dropped"
        domains={insights.topWarnDomains ?? []}
        dnsStatus={dnsStatus}
        onCheckDns={runDnsCheck}
        dnsLoading={dnsLoading}
        hasChecked={checked.length > 0}
      />

      {/* ── Verdict banner ────────────────────────────────────────── */}
      {verdict && (
        <div className={`rounded-xl border px-4 py-3 flex items-start gap-2.5 ${
          verdict.tone === "good" ? "border-emerald-200 bg-emerald-50" :
          verdict.tone === "warn" ? "border-amber-200 bg-amber-50" :
          "border-zinc-200 bg-zinc-50"
        }`}>
          {verdict.tone === "good"
            ? <CheckCircle2 size={16} className="text-emerald-600 shrink-0 mt-0.5" />
            : <AlertTriangle size={16} className="text-amber-600 shrink-0 mt-0.5" />}
          <div className={`text-xs leading-relaxed ${verdict.tone === "good" ? "text-emerald-900" : "text-amber-900"}`}>
            {verdict.text}
          </div>
        </div>
      )}

      {/* ── Rejected domains ──────────────────────────────────────── */}
      {(insights.topRejectedDomains ?? []).length > 0 && (
        <DomainTable
          title="Hard-rejected at RCPT"
          subtitle="Blocked before queueing — bad_rcpt table hits and any already-enforced checks"
          domains={insights.topRejectedDomains ?? []}
          dnsStatus={dnsStatus}
        />
      )}

      {/* ── Reasons + recent events ───────────────────────────────── */}
      <div className="grid grid-cols-2 gap-4">
        {(insights.reasons ?? []).length > 0 && (
          <div className="rounded-xl border border-zinc-100 bg-white p-4">
            <h3 className="text-sm font-semibold text-zinc-800 mb-3">Warn reasons</h3>
            <div className="space-y-2">
              {(insights.reasons ?? []).map((r) => (
                <div key={r.reason}>
                  <div className="flex justify-between text-xs mb-0.5">
                    <span className="text-zinc-700 truncate mr-2">{r.reason}</span>
                    <span className="text-zinc-400 tabular-nums shrink-0">{r.count} · {r.percent}%</span>
                  </div>
                  <div className="h-1.5 rounded-full bg-zinc-100 overflow-hidden">
                    <div className="h-full rounded-full bg-amber-400" style={{ width: `${r.percent}%` }} />
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        <div className="rounded-xl border border-zinc-100 bg-white p-4">
          <h3 className="text-sm font-semibold text-zinc-800 mb-3">Recent events</h3>
          <div className="max-h-72 overflow-y-auto">
            <table className="w-full text-xs">
              <thead className="text-zinc-400 text-left sticky top-0 bg-white">
                <tr>
                  <th className="py-1.5 pr-2 font-medium">Time</th>
                  <th className="py-1.5 pr-2 font-medium">Kind</th>
                  <th className="py-1.5 pr-2 font-medium">Recipient</th>
                  <th className="py-1.5 font-medium">Reason</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-50">
                {(insights.recent ?? []).map((ev, i) => (
                  <tr key={i} className="align-top">
                    <td className="py-1.5 pr-2 text-zinc-500 whitespace-nowrap tabular-nums">{fmtTime(ev.timestamp)}</td>
                    <td className="py-1.5 pr-2">
                      <span className={`px-1.5 py-0.5 rounded text-[10px] font-semibold ${
                        ev.kind === "warned" ? "bg-amber-50 text-amber-700" : "bg-rose-50 text-rose-700"
                      }`}>
                        {ev.kind === "warned" ? "WARN" : "REJECT"}
                      </span>
                    </td>
                    <td className="py-1.5 pr-2 text-zinc-700 break-all max-w-44">{ev.to}</td>
                    <td className="py-1.5 text-zinc-500">{ev.reason?.replace(/^Recipient address rejected: /, "")}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  );
}

// ──────────────────────────────────────────────
// Domain table with optional live DNS verdicts
// ──────────────────────────────────────────────

function DomainTable({ title, subtitle, domains, dnsStatus, onCheckDns, dnsLoading, hasChecked }: {
  title: string;
  subtitle: string;
  domains: WarnDomainStat[];
  dnsStatus: Record<string, DomainDNSStatus>;
  onCheckDns?: () => void;
  dnsLoading?: boolean;
  hasChecked?: boolean;
}) {
  if (domains.length === 0) return null;
  return (
    <div className="rounded-xl border border-zinc-100 bg-white p-4">
      <div className="flex items-center justify-between mb-1">
        <h3 className="text-sm font-semibold text-zinc-800">{title}</h3>
        {onCheckDns && (
          <button
            onClick={onCheckDns}
            disabled={dnsLoading}
            className="flex items-center gap-1.5 text-xs px-2.5 py-1.5 rounded-lg border border-zinc-200 hover:bg-zinc-50 disabled:opacity-50 transition-colors"
          >
            {dnsLoading
              ? <RefreshCw size={12} className="animate-spin" />
              : <Globe size={12} />}
            {dnsLoading ? "Checking DNS…" : hasChecked ? "Re-check DNS" : "Check DNS now"}
          </button>
        )}
      </div>
      <p className="text-xs text-zinc-400 mb-3">{subtitle}</p>
      <table className="w-full text-xs">
        <thead className="text-zinc-400 text-left">
          <tr>
            <th className="py-1.5 pr-2 font-medium">Domain</th>
            <th className="py-1.5 pr-2 font-medium text-right">Warned</th>
            <th className="py-1.5 pr-2 font-medium text-right">Rejected</th>
            <th className="py-1.5 pr-2 font-medium text-right">RCPTs</th>
            <th className="py-1.5 pr-2 font-medium">Last seen</th>
            <th className="py-1.5 font-medium">DNS verdict</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-zinc-50">
          {domains.map((d) => {
            const dns = dnsStatus[d.domain];
            const pill = dns ? DNS_PILL[dns.status] : null;
            return (
              <tr key={d.domain} title={pill?.hint}>
                <td className="py-1.5 pr-2 text-zinc-700 font-medium">{d.domain}</td>
                <td className="py-1.5 pr-2 text-right text-amber-600 tabular-nums">{d.warned > 0 ? d.warned.toLocaleString() : "—"}</td>
                <td className="py-1.5 pr-2 text-right text-rose-600 tabular-nums">{d.rejected > 0 ? d.rejected.toLocaleString() : "—"}</td>
                <td className="py-1.5 pr-2 text-right text-zinc-500 tabular-nums">{d.uniqueRcpts.toLocaleString()}</td>
                <td className="py-1.5 pr-2 text-zinc-400 whitespace-nowrap">{fmtTime(d.lastSeen)}</td>
                <td className="py-1.5">
                  {dns ? (
                    <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded font-semibold"
                      style={{ background: pill?.bg, color: pill?.fg }}>
                      {dns.status === "tempfail" ? <HelpCircle size={10} /> : null}
                      {pill?.label}
                    </span>
                  ) : (
                    <span className="text-zinc-300">—</span>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
