import { useEffect, useState, useCallback } from "react";
import { Search, ChevronDown, ChevronRight, Users, ChevronLeft, AlertTriangle } from "lucide-react";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";
import { EmptyState } from "./EmptyState";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { NativeSelect } from "@/components/ui/select";

interface QueueSummary {
  timestamp: number;
  queueId: string;
  from: string;
  recipients: string[];
  recipCount: number;
  nrcpt: number;
  size: number;
  finalStatus: string;
  sent: number;
  deferred: number;
  bounced: number;
}

interface PaginationInfo {
  page: number;
  limit: number;
  total: number;
  totalPages: number;
  hasNext: boolean;
  hasPrev: boolean;
}

interface Props {
  sinceHours: number;
  refreshKey?: number;
}

const PAGE_SIZE = 100;

const STATUS_COLOR: Record<string, string> = {
  sent: "#16a34a", deferred: "#d97706", bounced: "#e11d48", unknown: "#71717a",
};
const STATUS_BG: Record<string, string> = {
  sent: "#f0fdf4", deferred: "#fffbeb", bounced: "#fff1f2", unknown: "#f4f4f5",
};
const STATUS_LABEL: Record<string, string> = {
  sent: "Sent", deferred: "Deferred", bounced: "Bounced", unknown: "Truncated",
};

// Status filter — "unknown" surfaced as "Truncated"
type StatusTab = "all" | "sent" | "deferred" | "bounced" | "unknown";

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
  const label = STATUS_LABEL[status] ?? status;
  return (
    <span style={{
      fontSize: 10, padding: "1px 7px", borderRadius: 4, fontWeight: 600,
      background: STATUS_BG[status] ?? "#f4f4f5",
      color: STATUS_COLOR[status] ?? "#71717a",
    }}>{label}</span>
  );
}

export function QueueSummaryTable({ sinceHours, refreshKey }: Props) {
  const [rows, setRows] = useState<QueueSummary[]>([]);
  const [pagination, setPagination] = useState<PaginationInfo>({
    page: 1, limit: PAGE_SIZE, total: 0, totalPages: 1, hasNext: false, hasPrev: false,
  });
  const [unknownCount, setUnknownCount] = useState(0);
  const [bannerDismissed, setBannerDismissed] = useState(false);
  const [loading, setLoading] = useState(false);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusTab>("all");
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set());

  const load = useCallback((page: number, filter: StatusTab = statusFilter) => {
    setLoading(true);
    setExpandedRows(new Set());
    // Pass filter to backend so pagination is correct per-status
    const backendFilter = filter === "all" ? "" : filter;
    (LogService.GetQueueSummaries(sinceHours, page, PAGE_SIZE, "", backendFilter) as any)
      .then((r: any) => {
        setRows(r?.rows ?? []);
        if (r?.pagination) setPagination(r.pagination);
        if (typeof r?.unknownCount === "number") setUnknownCount(r.unknownCount);
        setLoading(false);
      })
      .catch(() => { setRows([]); setLoading(false); });
  }, [sinceHours, statusFilter]);

  // Reset to page 1 whenever sinceHours or refreshKey changes; also reset banner
  useEffect(() => {
    setBannerDismissed(false);
    load(1);
  }, [sinceHours, refreshKey]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleFilterChange = (s: StatusTab) => {
    setStatusFilter(s);
    load(1, s);
  };

  const toggleRow = (qid: string) => {
    setExpandedRows(prev => {
      const next = new Set(prev);
      next.has(qid) ? next.delete(qid) : next.add(qid);
      return next;
    });
  };

  // Search is still client-side on the current page (lightweight; full-text backend search is future work)
  const filtered = rows.filter(r => {
    if (!search) return true;
    const q = search.toLowerCase();
    return r.queueId.toLowerCase().includes(q) ||
           r.from.toLowerCase().includes(q) ||
           (r.recipients ?? []).some(t => t.toLowerCase().includes(q));
  });

  const start = (pagination.page - 1) * pagination.limit + 1;
  const end   = Math.min(pagination.page * pagination.limit, pagination.total);

  return (
    <div className="flex flex-col h-full bg-white">
      {/* Rotation banner — shown when any queues in this file have no delivery outcome */}
      {unknownCount > 0 && !bannerDismissed && (
        <div className="shrink-0 flex items-center gap-3 px-5 py-2.5 bg-amber-50 border-b border-amber-100 text-amber-800 text-xs">
          <AlertTriangle size={13} className="shrink-0 text-amber-500" />
          <span>
            <strong>{unknownCount} queue{unknownCount !== 1 ? "s" : ""}</strong> in this file
            {unknownCount === 1 ? " has" : " have"} no delivery outcome — smtp line is likely in a
            newer rotated log file. Load the next file to resolve them.
          </span>
          <button
            onClick={() => setBannerDismissed(true)}
            className="ml-auto shrink-0 text-amber-500 hover:text-amber-700 transition-colors font-medium"
          >
            Dismiss
          </button>
        </div>
      )}

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
        <NativeSelect
          value={statusFilter}
          onChange={e => handleFilterChange(e.target.value as StatusTab)}
          className="text-xs"
        >
          <option value="all">All</option>
          <option value="sent">Sent</option>
          <option value="deferred">Deferred</option>
          <option value="bounced">Bounced</option>
          <option value="unknown">{unknownCount > 0 ? `Truncated (${unknownCount})` : "Truncated"}</option>
        </NativeSelect>
      </div>

      {/* Table */}
      <div className="flex-1 overflow-y-auto">
        {loading && filtered.length === 0 ? (
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
                const isTruncated = r.finalStatus === "unknown";
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
                    <td className="px-4 py-2.5 max-w-50">
                      <span className="truncate block text-zinc-700" title={r.from}>{r.from || "—"}</span>
                    </td>
                    <td className="px-4 py-2.5">
                      <div className="flex items-center gap-1.5">
                        <Users size={11} className="text-zinc-300 shrink-0" />
                        {isTruncated && r.recipCount === 0 && r.nrcpt > 0 ? (
                          // Show nrcpt as a hint when no smtp delivery line was found
                          <span
                            className="text-zinc-400 tabular-nums"
                            title={`${r.nrcpt} recipient${r.nrcpt !== 1 ? "s" : ""} expected (from queue manager line) — delivery line in another log file`}
                          >
                            ~{r.nrcpt}
                          </span>
                        ) : (
                          <>
                            <span className="text-zinc-600 font-medium tabular-nums">{r.recipCount}</span>
                            {r.recipCount > 1 && (
                              <span className="text-zinc-400">
                                {r.recipients?.[0]?.split("@")[1] && `@${r.recipients[0].split("@")[1]}`}
                              </span>
                            )}
                          </>
                        )}
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
                        {isTruncated ? (
                          <div className="flex items-start gap-2 text-xs text-amber-700">
                            <AlertTriangle size={12} className="shrink-0 mt-0.5 text-amber-400" />
                            <span>
                              Delivery line not found in this log file.
                              {r.nrcpt > 0 && <> Queue manager reported <strong>{r.nrcpt} recipient{r.nrcpt !== 1 ? "s" : ""}</strong>.</>}
                              {" "}Load the next rotated log file to see the full delivery outcome.
                            </span>
                          </div>
                        ) : (
                          <>
                            <div className="text-xs text-zinc-400 mb-1.5">
                              {r.recipCount} recipient{r.recipCount !== 1 ? "s" : ""}
                            </div>
                            <div className="grid grid-cols-2 gap-x-8 gap-y-0.5">
                              {(r.recipients ?? []).map(rec => (
                                <span key={rec} className="text-xs text-zinc-600 truncate">{rec}</span>
                              ))}
                            </div>
                          </>
                        )}
                      </td>
                    </tr>
                  ),
                ];
              })}
            </tbody>
          </table>
        )}
      </div>

      {/* Footer with pagination */}
      <div className="shrink-0 flex items-center justify-between px-5 py-2 border-t border-zinc-100 bg-zinc-50/60">
        <span className="text-xs text-zinc-400">
          {search
            ? filtered.length > 0
              ? <><strong className="text-zinc-600">{filtered.length}</strong> result{filtered.length !== 1 ? "s" : ""} for "<strong className="text-zinc-600">{search}</strong>"</>
              : <>No results for "<strong className="text-zinc-600">{search}</strong>"{pagination.totalPages > 1 ? " on this page — try other pages" : ""}</>
            : pagination.total > 0
              ? <>Showing <strong className="text-zinc-600">{start}–{end}</strong> of <strong className="text-zinc-600">{pagination.total.toLocaleString()}</strong> queues</>
              : "No queues"}
        </span>
        {!search && pagination.totalPages > 1 && (
          <div className="flex items-center gap-1">
            <Button variant="ghost" size="icon-sm"
              disabled={!pagination.hasPrev || loading}
              onClick={() => load(pagination.page - 1)}>
              <ChevronLeft className="w-4 h-4" />
            </Button>
            <span className="text-xs text-zinc-400 tabular-nums px-1">
              {pagination.page} / {pagination.totalPages}
            </span>
            <Button variant="ghost" size="icon-sm"
              disabled={!pagination.hasNext || loading}
              onClick={() => load(pagination.page + 1)}>
              <ChevronRight className="w-4 h-4" />
            </Button>
          </div>
        )}
      </div>
    </div>
  );
}
