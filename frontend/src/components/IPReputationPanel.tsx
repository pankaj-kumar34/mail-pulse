import { useEffect, useState, useCallback } from "react";
import { ArrowLeft, X, ShieldAlert } from "lucide-react";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";
import { EmptyState } from "./EmptyState";

interface BlocklistStat {
  ourIp: string;
  service: string;
  count: number;
  domains: string[];
}

interface BlockEvent {
  timestamp: number;
  queueId: string;
  ourIp: string;
  service: string;
  destination: string;
  recipient: string;
  message: string;
}

interface Props {
  sinceHours: number;
  refreshKey?: number;
}

function fmt(ms: number) {
  return new Date(ms).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function plural(n: number, word: string) {
  return `${n.toLocaleString()} ${word}${n === 1 ? "" : "s"}`;
}

function BlockEventDrilldown({ stat, sinceHours, onClose }: {
  stat: BlocklistStat; sinceHours: number; onClose: () => void;
}) {
  const [rows, setRows] = useState<BlockEvent[] | null>(null);

  useEffect(() => {
    (LogService.GetBlockEvents(stat.ourIp, stat.service, sinceHours) as any)
      .then((r: any) => setRows(r ?? []))
      .catch(() => setRows([]));
  }, [stat.ourIp, stat.service, sinceHours]);

  return (
    <div style={{ position: "absolute", inset: 0, background: "#fff", borderRadius: "inherit", display: "flex", flexDirection: "column", zIndex: 10 }}>
      <div style={{ display: "flex", alignItems: "center", gap: "10px", padding: "12px 16px", borderBottom: "1px solid #f4f4f5", flexShrink: 0 }}>
        <button onClick={onClose} style={{ display: "flex", alignItems: "center", gap: "4px", fontSize: "12px", color: "#71717a", background: "none", border: "none", cursor: "pointer" }}>
          <ArrowLeft size={13} /> Back
        </button>
        <span style={{ fontSize: "10px", padding: "1px 8px", borderRadius: "4px", background: "#fff1f2", color: "#e11d48", fontWeight: 600, fontFamily: "monospace" }}>{stat.ourIp}</span>
        <span style={{ fontSize: "12px", color: "#3f3f46", flex: 1, fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{stat.service}</span>
        {rows && <span style={{ fontSize: "11px", color: "#a1a1aa", flexShrink: 0 }}>{plural(rows.length, "event")}</span>}
        <button onClick={onClose} style={{ background: "none", border: "none", cursor: "pointer", color: "#a1a1aa", display: "flex", flexShrink: 0 }}><X size={14} /></button>
      </div>
      <div style={{ flex: 1, overflowY: "auto" }}>
        {!rows ? (
          <div style={{ padding: "24px", textAlign: "center", color: "#a1a1aa", fontSize: "13px" }}>Loading…</div>
        ) : rows.length === 0 ? (
          <div style={{ padding: "8px" }}><EmptyState icon="search" title="No block events found" compact /></div>
        ) : (
          <table style={{ width: "100%", fontSize: "11px", borderCollapse: "collapse" }}>
            <thead style={{ position: "sticky", top: 0 }}>
              <tr style={{ background: "#fafafa", borderBottom: "1px solid #f4f4f5" }}>
                {["Time", "Recipient", "Domain", "Server Response"].map(h => (
                  <th key={h} style={{ padding: "6px 12px", textAlign: "left", fontWeight: 600, color: "#a1a1aa", fontSize: "10px", letterSpacing: "0.06em", textTransform: "uppercase", whiteSpace: "nowrap" }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((r, i) => (
                <tr key={i} style={{ borderBottom: "1px solid #f4f4f5", background: i % 2 === 0 ? "#fff" : "#fafafa" }}>
                  <td style={{ padding: "6px 12px", color: "#a1a1aa", whiteSpace: "nowrap", fontVariantNumeric: "tabular-nums" }}>{fmt(r.timestamp)}</td>
                  <td style={{ padding: "6px 12px", maxWidth: "180px" }}>
                    <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: "#3f3f46" }} title={r.recipient}>{r.recipient || "—"}</span>
                  </td>
                  <td style={{ padding: "6px 12px", whiteSpace: "nowrap" }}>
                    <span style={{ padding: "1px 6px", borderRadius: "4px", background: "#f4f4f5", fontSize: "10px", color: "#71717a" }}>{r.destination || "—"}</span>
                  </td>
                  <td style={{ padding: "6px 12px", color: "#52525b", maxWidth: "280px" }}>
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

export function IPReputationPanel({ sinceHours, refreshKey }: Props) {
  const [stats, setStats] = useState<BlocklistStat[]>([]);
  const [drill, setDrill] = useState<BlocklistStat | null>(null);

  useEffect(() => {
    (LogService.GetIPReputationReport(sinceHours) as any)
      .then((r: any) => setStats(r?.stats ?? []))
      .catch(() => setStats([]));
    setDrill(null);
  }, [sinceHours, refreshKey]);

  const closeDrill = useCallback(() => setDrill(null), []);

  if (!stats.length) {
    return <EmptyState icon="check" title="No IP blocks detected" subtitle="Your sending IP is not on any known blocklist" compact />;
  }

  // Group by IP to show IP once as a header
  const byIP = stats.reduce<Record<string, BlocklistStat[]>>((acc, s) => {
    (acc[s.ourIp] ??= []).push(s);
    return acc;
  }, {});

  const maxCount = Math.max(...stats.map(s => s.count));

  return (
    <div style={{ position: "relative" }}>
      {drill && <BlockEventDrilldown stat={drill} sinceHours={sinceHours} onClose={closeDrill} />}

      {Object.entries(byIP).map(([ip, entries]) => (
        <div key={ip} className="mb-4">
          {/* IP header */}
          <div className="flex items-center gap-2 mb-2">
            <span className="text-xs font-mono font-semibold bg-zinc-100 text-zinc-500 px-2 py-0.5 rounded">{ip}</span>
            <span className="text-xs text-zinc-400">{plural(entries.reduce((a, e) => a + e.count, 0), "total block")}</span>
          </div>

          {/* Rows */}
          <div className="space-y-2">
            {entries.map((s) => {
              const barPct = maxCount > 0 ? (s.count / maxCount) * 100 : 0;
              return (
                <div key={s.service}
                  onClick={() => setDrill(s)}
                  className="cursor-pointer hover:bg-zinc-50 active:bg-zinc-100 transition-colors rounded-lg p-3 border border-zinc-100">

                  {/* Service name + count */}
                  <div className="flex items-center justify-between gap-2 mb-2">
                    <div className="flex items-center gap-1.5 min-w-0">
                      <ShieldAlert size={13} className="text-rose-400 shrink-0" />
                      <span className="text-xs font-medium text-zinc-700 leading-tight">{s.service}</span>
                    </div>
                    <span className="text-xs font-semibold text-rose-500 shrink-0 tabular-nums">
                      {plural(s.count, "block")}
                    </span>
                  </div>

                  {/* Volume bar */}
                  <div className="h-1 rounded-full bg-zinc-100 mb-2">
                    <div className="h-full rounded-full bg-rose-400" style={{ width: `${barPct}%` }} />
                  </div>

                  {/* Affected domains */}
                  <div className="flex flex-wrap gap-1">
                    {s.domains.slice(0, 6).map(d => (
                      <span key={d} className="text-xs bg-zinc-100 text-zinc-500 px-1.5 py-0.5 rounded leading-tight">{d}</span>
                    ))}
                    {s.domains.length > 6 && (
                      <span className="text-xs text-zinc-400 self-center">+{s.domains.length - 6} more</span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      ))}
    </div>
  );
}
