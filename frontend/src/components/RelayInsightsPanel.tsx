import { useEffect, useState, useCallback } from "react";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";
import { AlertTriangle, ShieldOff, CheckCircle2, ArrowLeft, X } from "lucide-react";

interface RelayStat {
  relay: string; sent: number; deferred: number; bounced: number;
  total: number; failRate: number; kind: string; topDsn: string; lastMessage: string;
}
interface RelayInsights { throttling: RelayStat[]; blocking: RelayStat[]; healthy: RelayStat[]; }
interface RelayMessage  { timestamp: number; to: string; status: string; dsn: string; message: string; }

// ── Status badge ────────────────────────────────────────────────────────────
const STATUS_COLOR: Record<string, string> = { sent: "#10b981", deferred: "#d97706", bounced: "#e11d48" };
const STATUS_BG:    Record<string, string> = { sent: "#f0fdf4", deferred: "#fffbeb", bounced: "#fff1f2" };

function fmt(ms: number) {
  return new Date(ms).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

// ── Message drilldown overlay ────────────────────────────────────────────────
function RelayMessageDrilldown({ relay, status, sinceHours, onClose }: {
  relay: string; status: string; sinceHours: number; onClose: () => void;
}) {
  const [rows, setRows] = useState<RelayMessage[] | null>(null);

  useEffect(() => {
    LogService.GetRelayMessages(relay, status, sinceHours)
      .then((r: any) => setRows(r ?? []))
      .catch(() => setRows([]));
  }, [relay, status, sinceHours]);

  const label = status === "deferred" ? "Deferred" : status === "bounced" ? "Bounced" : "All";

  return (
    <div style={{ position: "absolute", inset: 0, background: "#fff", borderRadius: "inherit", display: "flex", flexDirection: "column", zIndex: 10 }}>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", gap: "10px", padding: "12px 16px", borderBottom: "1px solid #f4f4f5", flexShrink: 0 }}>
        <button onClick={onClose} style={{ display: "flex", alignItems: "center", gap: "4px", fontSize: "12px", color: "#71717a", background: "none", border: "none", cursor: "pointer" }}>
          <ArrowLeft size={13} /> Back
        </button>
        <span style={{ fontSize: "12px", fontFamily: "monospace", color: "#18181b", flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{relay}</span>
        <span style={{ fontSize: "11px", padding: "1px 8px", borderRadius: "4px", background: STATUS_BG[status] ?? "#f4f4f5", color: STATUS_COLOR[status] ?? "#71717a", fontWeight: 600 }}>{label}</span>
        {rows && <span style={{ fontSize: "11px", color: "#a1a1aa" }}>{rows.length} messages</span>}
        <button onClick={onClose} style={{ background: "none", border: "none", cursor: "pointer", color: "#a1a1aa", display: "flex" }}><X size={14} /></button>
      </div>
      {/* Table */}
      <div style={{ flex: 1, overflowY: "auto" }}>
        {!rows ? (
          <div style={{ padding: "24px", textAlign: "center", color: "#a1a1aa", fontSize: "13px" }}>Loading…</div>
        ) : rows.length === 0 ? (
          <div style={{ padding: "24px", textAlign: "center", color: "#a1a1aa", fontSize: "13px" }}>No messages</div>
        ) : (
          <table style={{ width: "100%", fontSize: "11px", borderCollapse: "collapse" }}>
            <thead style={{ position: "sticky", top: 0 }}>
              <tr style={{ background: "#fafafa", borderBottom: "1px solid #f4f4f5" }}>
                {["Time", "Recipient", "Status", "DSN", "Server Response"].map((h, i) => (
                  <th key={h} style={{ padding: "6px 12px", textAlign: "left", fontWeight: 600, color: "#a1a1aa", fontSize: "10px", letterSpacing: "0.06em", textTransform: "uppercase", whiteSpace: "nowrap" }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((r, i) => (
                <tr key={i} style={{ borderBottom: "1px solid #f4f4f5", background: i % 2 === 0 ? "#fff" : "#fafafa" }}>
                  <td style={{ padding: "6px 12px", color: "#a1a1aa", whiteSpace: "nowrap", fontVariantNumeric: "tabular-nums" }}>{fmt(r.timestamp)}</td>
                  <td style={{ padding: "6px 12px", maxWidth: "180px" }}>
                    <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: "#3f3f46" }} title={r.to}>{r.to}</span>
                  </td>
                  <td style={{ padding: "6px 12px", whiteSpace: "nowrap" }}>
                    <span style={{ padding: "1px 7px", borderRadius: "4px", background: STATUS_BG[r.status] ?? "#f4f4f5", color: STATUS_COLOR[r.status] ?? "#71717a", fontWeight: 500 }}>{r.status}</span>
                  </td>
                  <td style={{ padding: "6px 12px", whiteSpace: "nowrap" }}>
                    <span style={{ padding: "1px 6px", borderRadius: "4px", background: "#f4f4f5", fontFamily: "monospace", fontSize: "10px", color: "#71717a" }}>{r.dsn || "—"}</span>
                  </td>
                  <td style={{ padding: "6px 12px", color: "#52525b", maxWidth: "300px" }}>
                    <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={r.message}>{r.message || "—"}</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

// ── Clickable count cell ─────────────────────────────────────────────────────
function CountCell({ value, color, onClick }: { value: number; color: string; onClick: () => void }) {
  if (value === 0) return <span style={{ color: "#e4e4e7" }}>—</span>;
  return (
    <button onClick={onClick} style={{
      background: "none", border: "none", cursor: "pointer",
      color, fontVariantNumeric: "tabular-nums", fontSize: "11px", fontWeight: 600,
      textDecoration: "underline", textDecorationStyle: "dotted", textUnderlineOffset: "3px",
      padding: 0,
    }}>
      {value.toLocaleString()}
    </button>
  );
}

// ── Relay table ──────────────────────────────────────────────────────────────
function RelayTable({ rows, maxFail, onDrill }: {
  rows: RelayStat[]; maxFail: number;
  onDrill: (relay: string, status: string) => void;
}) {
  if (!rows.length) return <p className="text-zinc-400 text-sm px-4 py-3">None detected</p>;
  return (
    <table style={{ width: "100%", fontSize: "11px", borderCollapse: "collapse" }}>
      <thead style={{ position: "sticky", top: 0 }}>
        <tr style={{ background: "#fafafa", borderBottom: "1px solid #f4f4f5" }}>
          {["Relay Host", "Sent", "Deferred", "Bounced", "Fail %"].map((h, i) => (
            <th key={h} style={{
              padding: "6px 12px", textAlign: i >= 1 ? "right" : "left",
              fontWeight: 600, color: "#a1a1aa", fontSize: "10px",
              letterSpacing: "0.06em", textTransform: "uppercase", whiteSpace: "nowrap",
            }}>{h}</th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((r, i) => {
          const barPct  = maxFail > 0 ? ((r.deferred + r.bounced) / maxFail) * 100 : 0;
          const barColor = r.kind === "blocking" ? "#f43f5e" : r.kind === "ok" ? "#10b981" : "#eab308";
          const failColor = r.failRate >= 80 ? "#e11d48" : r.failRate >= 50 ? "#d97706" : "#059669";
          return (
            <tr key={r.relay} style={{ borderBottom: "1px solid #f4f4f5", background: i % 2 === 0 ? "#fff" : "#fafafa" }}>
              <td style={{ padding: "7px 12px", maxWidth: "260px" }}>
                <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: "#3f3f46", fontFamily: "monospace", fontSize: "11px" }}
                  title={r.relay}>{r.relay}</span>
                <div style={{ height: "3px", background: "#f4f4f5", borderRadius: "2px", marginTop: "3px" }}>
                  <div style={{ height: "100%", width: `${barPct}%`, background: barColor, borderRadius: "2px" }} />
                </div>
              </td>
              <td style={{ padding: "7px 12px", textAlign: "right" }}>
                <CountCell value={r.sent}     color="#10b981" onClick={() => onDrill(r.relay, "sent")} />
              </td>
              <td style={{ padding: "7px 12px", textAlign: "right" }}>
                <CountCell value={r.deferred} color="#d97706" onClick={() => onDrill(r.relay, "deferred")} />
              </td>
              <td style={{ padding: "7px 12px", textAlign: "right" }}>
                <CountCell value={r.bounced}  color="#e11d48" onClick={() => onDrill(r.relay, "bounced")} />
              </td>
              <td style={{ padding: "7px 12px", textAlign: "right", fontWeight: 600, color: failColor, fontVariantNumeric: "tabular-nums" }}>
                {r.failRate.toFixed(1)}%
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

// ── Section card ─────────────────────────────────────────────────────────────
function Section({ title, subtitle, icon: Icon, iconBg, iconColor, children }: {
  title: string; subtitle: string; icon: React.ElementType;
  iconBg: string; iconColor: string; children: React.ReactNode;
}) {
  return (
    <div className="rounded-xl border border-zinc-100 bg-white flex flex-col" style={{ position: "relative", flex: "1 1 0", minHeight: 0 }}>
      <div className="px-5 py-3.5 border-b border-zinc-100 flex items-center gap-3 shrink-0">
        <div style={{ background: iconBg, borderRadius: "8px", padding: "6px", display: "flex" }}>
          <Icon size={14} style={{ color: iconColor }} />
        </div>
        <div>
          <h3 className="text-sm font-semibold text-zinc-900">{title}</h3>
          <p className="text-xs text-zinc-400">{subtitle}</p>
        </div>
      </div>
      <div className="overflow-auto flex-1" style={{ minHeight: 0 }}>
        {children}
      </div>
    </div>
  );
}

// ── Main panel ───────────────────────────────────────────────────────────────
export function RelayInsightsPanel({ sinceHours, refreshKey }: { sinceHours: number; refreshKey: number }) {
  const [data, setData]       = useState<RelayInsights | null>(null);
  const [drill, setDrill]     = useState<{ relay: string; status: string } | null>(null);
  const [drillSection, setDrillSection] = useState<string>("");
  const [filter, setFilter]   = useState("");

  useEffect(() => {
    LogService.GetRelayInsights(sinceHours)
      .then((r: any) => setData(r))
      .catch(() => {});
  }, [sinceHours, refreshKey]);

  const drillThrottling = useCallback((relay: string, status: string) => { setDrill({ relay, status }); setDrillSection("throttling"); }, []);
  const drillBlocking   = useCallback((relay: string, status: string) => { setDrill({ relay, status }); setDrillSection("blocking"); }, []);
  const drillHealthy    = useCallback((relay: string, status: string) => { setDrill({ relay, status }); setDrillSection("healthy"); }, []);
  const closeDrill      = useCallback(() => setDrill(null), []);

  if (!data) return (
    <div className="flex-1 flex items-center justify-center text-zinc-400 text-sm">Loading…</div>
  );

  const q = filter.toLowerCase();
  const applyFilter = (rows: RelayStat[]) => q ? rows.filter(r => r.relay.toLowerCase().includes(q)) : rows;

  const throttled = applyFilter(data.throttling.slice(0, 50));
  const blocked   = applyFilter(data.blocking.slice(0, 50));
  const healthy   = applyFilter(data.healthy.slice(0, 50));

  const throttleMax = Math.max(...data.throttling.map(r => r.deferred + r.bounced), 1);
  const blockMax    = Math.max(...data.blocking.map(r => r.deferred + r.bounced), 1);
  const healthyMax  = Math.max(...data.healthy.map(r => r.deferred + r.bounced), 1);

  return (
    <div className="flex flex-col h-full" style={{ position: "relative" }}>
      {/* Search bar */}
      <div className="px-5 py-2.5 border-b border-zinc-100 bg-white shrink-0">
        <input
          value={filter} onChange={e => setFilter(e.target.value)}
          placeholder="Filter by relay hostname…"
          className="w-full text-sm px-3 py-1.5 rounded-lg border border-zinc-200 outline-none focus:border-zinc-400 bg-white text-zinc-700 placeholder:text-zinc-400"
        />
      </div>
      {/* Three equal-height scrollable cards */}
      <div className="flex flex-col flex-1 min-h-0 px-5 pt-3 pb-4 gap-4">

      <Section title="Throttling" subtitle="Relays deferring your mail — server busy, rate limits"
        icon={AlertTriangle} iconBg="#fffbeb" iconColor="#d97706">
        {drill && drillSection === "throttling" && (
          <RelayMessageDrilldown relay={drill.relay} status={drill.status} sinceHours={sinceHours} onClose={closeDrill} />
        )}
        <RelayTable rows={throttled} maxFail={throttleMax} onDrill={drillThrottling} />
      </Section>

      <Section title="Blocking" subtitle="Relays rejecting with policy / access denied codes"
        icon={ShieldOff} iconBg="#fff1f2" iconColor="#e11d48">
        {drill && drillSection === "blocking" && (
          <RelayMessageDrilldown relay={drill.relay} status={drill.status} sinceHours={sinceHours} onClose={closeDrill} />
        )}
        <RelayTable rows={blocked} maxFail={blockMax} onDrill={drillBlocking} />
      </Section>

      <Section title="Healthy Relays" subtitle="Delivering successfully"
        icon={CheckCircle2} iconBg="#f0fdf4" iconColor="#059669">
        {drill && drillSection === "healthy" && (
          <RelayMessageDrilldown relay={drill.relay} status={drill.status} sinceHours={sinceHours} onClose={closeDrill} />
        )}
        <RelayTable rows={healthy} maxFail={healthyMax} onDrill={drillHealthy} />
      </Section>

      </div>
    </div>
  );
}
