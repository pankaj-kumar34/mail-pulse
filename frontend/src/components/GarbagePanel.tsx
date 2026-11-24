import { useEffect, useState } from "react";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";
import { Trash2, ChevronDown, Search, Loader2 } from "lucide-react";
import { EmptyState } from "./EmptyState";
import { Input } from "@/components/ui/input";

interface GarbageRow {
  to: string; flags: string[]; senders: string[];
  sent: number; deferred: number; bounced: number;
  relays: string[]; lastSeen: number; lastDsn: string; lastMessage: string;
}

const FLAG_STYLE: Record<string, { bg: string; fg: string; label: string }> = {
  "reserved-tld":     { bg: "#fff1f2", fg: "#e11d48", label: "reserved tld" },
  "random-localpart": { bg: "#fffbeb", fg: "#d97706", label: "random username" },
  "user-unknown":     { bg: "#fef2f2", fg: "#dc2626", label: "user unknown" },
  "garbage-sender":   { bg: "#f5f3ff", fg: "#7c3aed", label: "garbage sender" },
  "unverified":       { bg: "#f4f4f5", fg: "#71717a", label: "unverified" },
};

function Flag({ f }: { f: string }) {
  const s = FLAG_STYLE[f] ?? { bg: "#f4f4f5", fg: "#71717a", label: f };
  return (
    <span style={{ padding: "1px 7px", borderRadius: "4px", background: s.bg, color: s.fg, fontWeight: 600, fontSize: "10px", whiteSpace: "nowrap" }}>
      {s.label}
    </span>
  );
}

function fmtTime(ms: number) {
  return new Date(ms).toLocaleTimeString("en-GB", { hour12: false });
}

export function GarbagePanel({ sinceHours, refreshKey }: { sinceHours: number; refreshKey: number }) {
  const [rows, setRows]       = useState<GarbageRow[] | null>(null);
  const [filter, setFilter]   = useState("");
  const [expanded, setExpanded] = useState<string | null>(null);

  useEffect(() => {
    LogService.GetGarbage(sinceHours)
      .then((r: any) => setRows(r ?? []))
      .catch(() => setRows([]));
  }, [sinceHours, refreshKey]);

  if (!rows) return (
    <div className="flex-1 flex items-center justify-center text-zinc-400 text-sm">
      <Loader2 className="w-5 h-5 animate-spin" style={{ color: "#d4d4d8" }} />
    </div>
  );

  const q = filter.toLowerCase();
  const filtered = q ? rows.filter(r =>
    r.to.toLowerCase().includes(q) || r.senders.some(s => s.toLowerCase().includes(q))
  ) : rows;

  const countBy = (f: string) => rows.filter(r => r.flags.includes(f)).length;
  const totalMail = rows.reduce((n, r) => n + r.sent + r.deferred + r.bounced, 0);

  return (
    <div className="rounded-xl border border-zinc-100 bg-white flex flex-col h-full overflow-hidden">
      {/* Header + flags summary */}
      <div className="px-5 py-3.5 border-b border-zinc-100 flex items-center gap-3 shrink-0">
        <div style={{ background: "#fff1f2", borderRadius: "8px", padding: "6px", display: "flex" }}>
          <Trash2 size={14} style={{ color: "#e11d48" }} />
        </div>
        <div className="flex-1">
          <h3 className="text-sm font-semibold text-zinc-800">Garbage Traffic</h3>
          <p className="text-xs text-zinc-400">Recipients that can never be real — QA fixtures, dead domains, user-unknown bounces</p>
        </div>
        <div className="flex items-center gap-1.5">
          <Flag f="reserved-tld" /> <span className="text-xs tabular-nums" style={{ color: "#71717a" }}>{countBy("reserved-tld")}</span>
          <Flag f="random-localpart" /> <span className="text-xs tabular-nums" style={{ color: "#71717a" }}>{countBy("random-localpart")}</span>
          <Flag f="user-unknown" /> <span className="text-xs tabular-nums" style={{ color: "#71717a" }}>{countBy("user-unknown")}</span>
          <Flag f="unverified" /> <span className="text-xs tabular-nums" style={{ color: "#71717a" }}>{countBy("unverified")}</span>
        </div>
      </div>

      {/* Search */}
      <div className="px-5 py-2.5 border-b border-zinc-100 shrink-0">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 pointer-events-none" style={{ color: "#a1a1aa" }} />
          <Input value={filter} onChange={e => setFilter(e.target.value)}
            placeholder="Filter by recipient or sender…" className="pl-10" />
        </div>
      </div>

      {/* Table */}
      <div className="overflow-auto flex-1" style={{ minHeight: 0 }}>
        {filtered.length === 0 ? (
          <EmptyState icon="mail" title={rows.length === 0 ? "No garbage detected" : "No matches"} subtitle={rows.length === 0 ? "Nothing flagged in this window" : "Try adjusting your filter"} compact />
        ) : (
          <table className="w-full text-sm table-fixed">
            <colgroup>
              <col style={{ width: "24%" }} />
              <col style={{ width: "26%" }} />
              <col style={{ width: "17%" }} />
              <col style={{ width: "9%" }} />
              <col style={{ width: "24%" }} />
            </colgroup>
            <thead style={{ position: "sticky", top: 0 }}>
              <tr style={{ background: "#fafafa", borderBottom: "1px solid #f4f4f5" }}>
                {["From", "Recipient", "Counts", "Last Seen", "Last Response"].map(h => (
                  <th key={h} className="px-4 py-2.5 text-left text-[11px] font-semibold uppercase tracking-wider whitespace-nowrap" style={{ color: "#a1a1aa" }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {filtered.map((r, i) => {
                const key = `${r.to}-${i}`;
                const isOpen = expanded === key;
                const total = r.sent + r.deferred + r.bounced;
                return [
                  <tr key={key} className="transition-colors hover:bg-zinc-50/60 cursor-pointer"
                    style={{ borderBottom: isOpen ? "none" : "1px solid #f4f4f5", background: i % 2 === 0 ? "#fff" : "#fafafa" }}
                    onClick={() => setExpanded(isOpen ? null : key)}>
                    {/* From */}
                    <td className="px-4 py-2.5 text-xs" style={{ color: "#52525b" }}>
                      <span className="block truncate font-mono" style={{ fontSize: "11px" }} title={r.senders.join(", ")}>
                        {r.senders.length === 0 ? "—" : r.senders.length === 1 ? r.senders[0] : `${r.senders[0]} +${r.senders.length - 1}`}
                      </span>
                      {r.flags.includes("garbage-sender") && (
                        <div className="mt-1"><Flag f="garbage-sender" /></div>
                      )}
                    </td>
                    {/* Recipient + flags — garbage-sender is a sender-side signal, shown under From */}
                    <td className="px-4 py-2.5">
                      <div className="flex items-center gap-1">
                        <ChevronDown className="w-3 h-3 shrink-0 transition-transform" style={{ color: "#d4d4d8", transform: isOpen ? "rotate(0deg)" : "rotate(-90deg)" }} />
                        <span className="font-mono text-xs truncate" style={{ color: "#3f3f46" }} title={r.to}>{r.to}</span>
                      </div>
                      <div className="flex gap-1 mt-1 ml-4 flex-wrap">
                        {r.flags.filter(f => f !== "garbage-sender").map(f => <Flag key={f} f={f} />)}
                      </div>
                    </td>
                    {/* Counts */}
                    <td className="px-4 py-2.5 text-xs tabular-nums whitespace-nowrap">
                      <span style={{ color: "#10b981" }}>{r.sent}</span>
                      <span style={{ color: "#d4d4d8" }}> / </span>
                      <span style={{ color: "#d97706" }}>{r.deferred}</span>
                      <span style={{ color: "#d4d4d8" }}> / </span>
                      <span style={{ color: "#e11d48" }}>{r.bounced}</span>
                      <span style={{ color: "#a1a1aa", fontSize: "10px" }}> · {total.toLocaleString()}</span>
                    </td>
                    {/* Last seen */}
                    <td className="px-4 py-2.5 text-xs tabular-nums whitespace-nowrap" style={{ color: "#71717a" }}>
                      {fmtTime(r.lastSeen)}
                    </td>
                    {/* Last response */}
                    <td className="px-4 py-2.5 text-xs" style={{ color: "#52525b" }}>
                      <span className="block truncate" title={r.lastMessage}>
                        {r.lastDsn && <span className="font-mono mr-1.5" style={{ padding: "0 4px", borderRadius: "3px", background: "#f4f4f5", fontSize: "10px", color: "#71717a" }}>{r.lastDsn}</span>}
                        {r.lastMessage || "—"}
                      </span>
                    </td>
                  </tr>,
                  isOpen && (
                    <tr key={`${key}-detail`} style={{ background: "#fafafa", borderBottom: "1px solid #f4f4f5" }}>
                      <td colSpan={5} className="px-4 py-3">
                        <div className="grid grid-cols-2 gap-x-6 gap-y-1.5 text-xs">
                          <div>
                            <span className="font-semibold uppercase tracking-wider" style={{ color: "#a1a1aa", fontSize: "10px" }}>All senders</span>
                            <div className="font-mono mt-0.5" style={{ color: "#52525b", wordBreak: "break-all" }}>
                              {r.senders.length ? r.senders.join(", ") : "—"}
                            </div>
                          </div>
                          <div>
                            <span className="font-semibold uppercase tracking-wider" style={{ color: "#a1a1aa", fontSize: "10px" }}>Relays used</span>
                            <div className="font-mono mt-0.5" style={{ color: "#52525b", wordBreak: "break-all" }}>
                              {r.relays.length ? r.relays.join(", ") : "—"}
                            </div>
                          </div>
                          <div className="col-span-2">
                            <span className="font-semibold uppercase tracking-wider" style={{ color: "#a1a1aa", fontSize: "10px" }}>Last server response</span>
                            <div className="mt-0.5" style={{ color: "#52525b", wordBreak: "break-word" }}>
                              {r.lastDsn && <span className="font-mono mr-1.5" style={{ padding: "0 4px", borderRadius: "3px", background: "#f4f4f5", fontSize: "10px", color: "#71717a" }}>{r.lastDsn}</span>}
                              {r.lastMessage || <span style={{ color: "#d4d4d8" }}>No response recorded</span>}
                            </div>
                          </div>
                        </div>
                      </td>
                    </tr>
                  )
                ];
              })}
            </tbody>
          </table>
        )}
      </div>

      {/* Footer */}
      <div className="px-5 py-3 shrink-0 flex items-center justify-between" style={{ borderTop: "1px solid #e4e4e7", background: "#fafafa" }}>
        <span className="text-xs" style={{ color: "#a1a1aa" }}>
          {filtered.length.toLocaleString()} flagged recipients · {totalMail.toLocaleString()} mails
        </span>
        <span className="text-xs" style={{ color: "#d4d4d8" }}>sorted by volume</span>
      </div>
    </div>
  );
}
