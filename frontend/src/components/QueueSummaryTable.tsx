import { useEffect, useState, useCallback } from "react";
import { Search, ChevronDown, ChevronRight, Users } from "lucide-react";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";
import { EmptyState } from "./EmptyState";
import { Input } from "@/components/ui/input";

interface QueueSummary {
  timestamp: number;
  queueId: string;
  from: string;
  recipients: string[];
  recipCount: number;
  size: number;
  finalStatus: string;
  sent: number;
  deferred: number;
  bounced: number;
}

interface Props {
  sinceHours: number;
  refreshKey?: number;
}

const STATUS_COLOR: Record<string, string> = {
  sent: "#16a34a", deferred: "#d97706", bounced: "#e11d48",
};
const STATUS_BG: Record<string, string> = {
  sent: "#f0fdf4", deferred: "#fffbeb", bounced: "#fff1f2",
};

function fmtTime(ms: number) {
  return new Date(ms).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function fmtSize(bytes: number): string {
  if (!bytes) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

function StatusPill({ status }: { status: string }) {
  return (
    <span style={{
      fontSize: 10, padding: "1px 7px", borderRadius: 4, fontWeight: 600,
      background: STATUS_BG[status] ?? "#f4f4f5",
      color: STATUS_COLOR[status] ?? "#71717a",
    }}>{status}</span>
  );
}

export function QueueSummaryTable({ sinceHours, refreshKey }: Props) {
  const [rows, setRows] = useState<QueueSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set());

  const load = useCallback(() => {
    setLoading(true);
    (LogService.GetQueueSummaries(sinceHours, "") as any)
      .then((r: any) => { setRows(r ?? []); setLoading(false); })
      .catch(() => { setRows([]); setLoading(false); });
  }, [sinceHours]);

  useEffect(() => { load(); }, [load, refreshKey]);

  const toggleRow = (qid: string) => {
    setExpandedRows(prev => {
      const next = new Set(prev);
      next.has(qid) ? next.delete(qid) : next.add(qid);
      return next;
    });
  };

  const filtered = rows.filter(r => {
    if (statusFilter !== "all" && r.finalStatus !== statusFilter) return false;
    if (search) {
      const q = search.toLowerCase();
      return r.queueId.toLowerCase().includes(q) ||
             r.from.toLowerCase().includes(q) ||
             r.recipients.some(t => t.toLowerCase().includes(q));
    }
    return true;
  });

  return (
    <div className="flex flex-col h-full bg-white">
      {/* Toolbar */}
      <div className="shrink-0 flex items-center gap-3 px-5 py-3 border-b border-zinc-100">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 pointer-events-none" style={{ color: "#a1a1aa" }} />
          <Input
            placeholder="Search queue ID, sender, recipient…"
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="pl-10"
          />
        </div>
        <div className="inline-flex items-center gap-0.5 rounded-lg p-0.5 bg-zinc-100 h-9">
          {["all", "sent", "deferred", "bounced"].map(s => (
            <button key={s}
              onClick={() => setStatusFilter(s)}
              className={`px-3 py-1.5 text-xs rounded-md transition-colors font-medium ${
                statusFilter === s ? "bg-white text-zinc-800 shadow-sm" : "text-zinc-500 hover:text-zinc-700"
              }`}>{s === "all" ? "All" : s.charAt(0).toUpperCase() + s.slice(1)}</button>
          ))}
        </div>
        <span className="text-xs text-zinc-400 shrink-0">{filtered.length.toLocaleString()} queues</span>
      </div>

      {/* Table */}
      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="flex items-center justify-center h-32 text-zinc-400 text-sm">Loading…</div>
        ) : filtered.length === 0 ? (
          <div className="p-8"><EmptyState icon="inbox" title="No queues found" subtitle="Load a log file or adjust filters" compact /></div>
        ) : (
          <table className="w-full text-xs border-collapse">
            <thead className="sticky top-0 bg-zinc-50 z-10">
              <tr className="border-b border-zinc-100">
                <th className="w-6 px-3 py-2" />
                <th className="px-4 py-2 text-left font-semibold text-zinc-400 uppercase tracking-wide text-[10px]">Time</th>
                <th className="px-4 py-2 text-left font-semibold text-zinc-400 uppercase tracking-wide text-[10px]">Queue ID</th>
                <th className="px-4 py-2 text-left font-semibold text-zinc-400 uppercase tracking-wide text-[10px]">From</th>
                <th className="px-4 py-2 text-left font-semibold text-zinc-400 uppercase tracking-wide text-[10px]">Recipients</th>
                <th className="px-4 py-2 text-right font-semibold text-zinc-400 uppercase tracking-wide text-[10px]">Size</th>
                <th className="px-4 py-2 text-left font-semibold text-zinc-400 uppercase tracking-wide text-[10px]">Status</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((r, i) => {
                const expanded = expandedRows.has(r.queueId);
                return [
                  <tr key={r.queueId}
                    onClick={() => toggleRow(r.queueId)}
                    className="border-b border-zinc-50 hover:bg-zinc-50 cursor-pointer transition-colors"
                    style={{ background: i % 2 === 0 ? "#fff" : "#fafafa" }}>
                    <td className="px-3 py-2.5 text-zinc-300">
                      {expanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
                    </td>
                    <td className="px-4 py-2.5 text-zinc-400 tabular-nums whitespace-nowrap">{fmtTime(r.timestamp)}</td>
                    <td className="px-4 py-2.5 font-mono text-zinc-600 whitespace-nowrap">{r.queueId}</td>
                    <td className="px-4 py-2.5 max-w-[200px]">
                      <span className="truncate block text-zinc-700" title={r.from}>{r.from || "—"}</span>
                    </td>
                    <td className="px-4 py-2.5">
                      <div className="flex items-center gap-1.5">
                        <Users size={11} className="text-zinc-300 shrink-0" />
                        <span className="text-zinc-600 font-medium tabular-nums">{r.recipCount}</span>
                        {r.recipCount > 1 && <span className="text-zinc-400">{r.recipients.slice(0, 1)[0]?.split("@")[1] && `@${r.recipients[0].split("@")[1]}`}</span>}
                      </div>
                    </td>
                    <td className="px-4 py-2.5 text-right text-zinc-400 tabular-nums whitespace-nowrap">{fmtSize(r.size)}</td>
                    <td className="px-4 py-2.5">
                      <div className="flex items-center gap-2">
                        <StatusPill status={r.finalStatus} />
                        {(r.deferred > 0 || r.bounced > 0) && (
                          <span className="text-zinc-300 text-[10px]">
                            {r.sent}✓ {r.deferred > 0 ? `${r.deferred}⟳` : ""} {r.bounced > 0 ? `${r.bounced}✗` : ""}
                          </span>
                        )}
                      </div>
                    </td>
                  </tr>,
                  expanded && (
                    <tr key={`${r.queueId}-expanded`} className="border-b border-zinc-100">
                      <td />
                      <td colSpan={6} className="pl-4 pr-8 py-3" style={{ background: "#f8fafc", borderLeft: "2px solid #e4e4e7" }}>
                        <div className="text-xs text-zinc-400 mb-1.5">
                          {r.recipCount} recipient{r.recipCount !== 1 ? "s" : ""}
                        </div>
                        <div className="grid grid-cols-2 gap-x-8 gap-y-0.5">
                          {r.recipients.map(rec => (
                            <span key={rec} className="text-xs text-zinc-600 truncate">{rec}</span>
                          ))}
                        </div>
                      </td>
                    </tr>
                  ),
                ];
              })}
            </tbody>
          </table>
        )}
      </div>

      {/* Footer */}
      <div className="shrink-0 flex items-center justify-between px-5 py-2 border-t border-zinc-100 bg-zinc-50/60">
        <span className="text-xs text-zinc-400">
          {filtered.reduce((a, r) => a + r.recipCount, 0).toLocaleString()} total recipients across {filtered.length.toLocaleString()} queues
        </span>
      </div>
    </div>
  );
}
