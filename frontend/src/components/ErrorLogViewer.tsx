import { useState, useEffect, useCallback, useMemo } from "react";
import { ChevronLeft, ChevronRight, Loader2, AlertCircle, AlertTriangle, Search } from "lucide-react";
import { GetErrors } from "../../bindings/github.com/pankajkumar/mail-pulse/logservice.js";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { EmptyState } from "./EmptyState";

interface LogEntry { id: string; timestamp: number; service: string; rawLog?: string; isError: boolean; isWarning: boolean }
interface Pagination { page: number; limit: number; total: number; totalPages: number; hasNext: boolean; hasPrev: boolean }
type FilterType = "all" | "errors" | "warnings";

export function ErrorLogViewer({ refreshKey, initialType = "all" }: { refreshKey: number; initialType?: FilterType }) {
  const [logs, setLogs]             = useState<LogEntry[]>([]);
  const [pagination, setPagination] = useState<Pagination>({ page: 1, limit: 20, total: 0, totalPages: 0, hasNext: false, hasPrev: false });
  const [type, setType]             = useState<FilterType>(initialType);
  const [search, setSearch]         = useState("");
  const [loading, setLoading]       = useState(false);

  useEffect(() => { setType(initialType); }, [initialType]);

  const fetchErrors = useCallback(async (page: number) => {
    setLoading(true);
    try {
      const r = await GetErrors(page, 20, type, "desc") as any;
      setLogs(r.logs ?? []);
      setPagination(r.pagination ?? { page: 1, limit: 20, total: 0, totalPages: 0, hasNext: false, hasPrev: false });
    } catch (e) { console.error(e); }
    finally { setLoading(false); }
  }, [type]);

  useEffect(() => { if (refreshKey === 0) return; fetchErrors(1); }, [fetchErrors, refreshKey]);

  const filtered = useMemo(() => {
    if (!search.trim()) return logs;
    const q = search.toLowerCase();
    return logs.filter(l => l.rawLog?.toLowerCase().includes(q));
  }, [logs, search]);

  const start = (pagination.page - 1) * pagination.limit + 1;
  const end   = Math.min(pagination.page * pagination.limit, pagination.total);
  const TABS: FilterType[] = ["all", "errors", "warnings"];

  return (
    <div className="flex flex-col h-full bg-white">

      {/* ── Toolbar — full width ── */}
      <div className="shrink-0 flex items-center gap-3 px-5 py-3 border-b border-zinc-100">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-zinc-400 pointer-events-none" />
          <Input placeholder="Search logs…" value={search} onChange={e => setSearch(e.target.value)} className="pl-10" />
        </div>
        <div className="inline-flex items-center gap-0.5 rounded-lg p-0.5 bg-zinc-100 h-9">
          {TABS.map(t => (
            <button key={t} onClick={() => { setType(t); setSearch(""); }}
              className="px-3 py-1.5 text-xs font-medium rounded-md transition-all cursor-pointer capitalize"
              style={type === t
                ? { background: "white", color: "#3f3f46" }
                : { background: "transparent", color: "#71717a" }}>
              {t}
            </button>
          ))}
        </div>
      </div>

      {/* ── Scrollable body — padded ── */}
      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="py-16 flex justify-center">
            <Loader2 className="w-5 h-5 animate-spin text-zinc-300" />
          </div>
        ) : filtered.length === 0 ? (
          <EmptyState icon="search" title={search ? "No matching logs" : "No errors or warnings"} subtitle={search ? "Try a different search term" : "Your log is clean"} fill />
        ) : (
          <div className="divide-y divide-zinc-100">
            {filtered.map(log => {
              const isErr = log.isError;
              const ts = new Date(log.timestamp).toLocaleString("en-GB", {
                day: "2-digit", month: "short", hour: "2-digit",
                minute: "2-digit", second: "2-digit", hour12: false,
              }).replace(",", "");
              return (
                <div key={log.id} className="px-5 py-3.5 hover:bg-zinc-50/70 transition-colors">
                  <div className="flex items-center gap-2 mb-1.5">
                    {isErr
                      ? <AlertCircle className="w-3.5 h-3.5 text-red-500 shrink-0" />
                      : <AlertTriangle className="w-3.5 h-3.5 text-amber-400 shrink-0" />}
                    <span className="text-xs text-zinc-400">{ts}</span>
                    <span className="ml-auto text-[10px] font-medium px-1.5 py-0.5 rounded-full"
                      style={isErr
                        ? { background: "#fee2e2", color: "#991b1b" }
                        : { background: "#fef3c7", color: "#78350f" }}>
                      {isErr ? "error" : "warning"}
                    </span>
                  </div>
                  {log.rawLog && (
                    <p className="select-text text-xs font-mono leading-relaxed pl-5 break-all whitespace-pre-wrap text-zinc-500">
                      {log.rawLog}
                    </p>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* ── Footer — full width ── */}
      {pagination.total > 0 && (
        <div className="shrink-0 flex items-center justify-between px-5 py-3 border-t border-zinc-100 bg-zinc-50/60">
          <span className="text-xs text-zinc-400">
            Showing <strong className="text-zinc-700">{start}–{end}</strong> of <strong className="text-zinc-700">{pagination.total}</strong>
          </span>
          <div className="flex items-center gap-2">
            <Button variant="outline" size="sm" disabled={!pagination.hasPrev} onClick={() => fetchErrors(pagination.page - 1)}>
              <ChevronLeft className="w-3.5 h-3.5" /> Prev
            </Button>
            <span className="text-xs text-zinc-400 tabular-nums">{pagination.page} / {pagination.totalPages}</span>
            <Button variant="outline" size="sm" disabled={!pagination.hasNext} onClick={() => fetchErrors(pagination.page + 1)}>
              Next <ChevronRight className="w-3.5 h-3.5" />
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
