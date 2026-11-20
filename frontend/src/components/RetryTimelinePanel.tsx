import { useEffect, useState, useCallback } from "react";
import { Search, ChevronRight, Clock, CheckCircle2, XCircle, RefreshCw, ShieldAlert, Inbox, Trash2, FileWarning } from "lucide-react";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";
import { EmptyState } from "./EmptyState";
import { Input } from "@/components/ui/input";
import { dsnExplain } from "../lib/dsn";

interface JourneyEvent {
  timestamp: number;
  kind: string; // "delivery" | "queued" | "removed" | "policy" | "log"
  status: string;
  dsn: string;
  relay: string;
  message: string;
}

interface JourneyQueue {
  queueId: string;
  from: string;
  to: string;
  finalStatus: string;
  events: JourneyEvent[];
}

interface JourneyResult {
  query: string;
  queues: JourneyQueue[];
  truncated: boolean;
}

interface Props {
  sinceHours: number;
  refreshKey?: number;
}

function fmtTime(ms: number) {
  return new Date(ms).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function fmtDelay(ms: number): string {
  if (ms <= 0) return "";
  const s = Math.round(ms / 1000);
  if (s < 60) return `+${s}s`;
  const m = Math.round(s / 60);
  if (m < 60) return `+${m}m`;
  return `+${Math.round(m / 60)}h`;
}

const STATUS_ICON: Record<string, React.ReactNode> = {
  sent:     <CheckCircle2 size={14} style={{ color: "#10b981", flexShrink: 0 }} />,
  bounced:  <XCircle      size={14} style={{ color: "#f43f5e", flexShrink: 0 }} />,
  deferred: <RefreshCw    size={14} style={{ color: "#eab308", flexShrink: 0 }} />,
  removed:  <Trash2       size={14} style={{ color: "#a1a1aa", flexShrink: 0 }} />,
  queued:   <Inbox        size={14} style={{ color: "#6366f1", flexShrink: 0 }} />,
  policy:   <ShieldAlert  size={14} style={{ color: "#e11d48", flexShrink: 0 }} />,
  log:      <FileWarning  size={14} style={{ color: "#a1a1aa", flexShrink: 0 }} />,
};

const STATUS_COLOR: Record<string, string> = {
  sent: "#10b981", bounced: "#f43f5e", deferred: "#eab308",
  removed: "#71717a", queued: "#6366f1", policy: "#e11d48",
  warned: "#d97706", rejected: "#e11d48", log: "#71717a",
};

function JourneyTimelineView({ queue, onBack }: { queue: JourneyQueue; onBack: () => void }) {
  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="shrink-0 px-5 py-3 border-b border-zinc-100">
        <button onClick={onBack} className="text-xs text-zinc-400 hover:text-zinc-600 mb-2 flex items-center gap-1">
          ← Back
        </button>
        <div className="flex items-center gap-2 flex-wrap">
          <span className="font-mono text-xs bg-zinc-100 text-zinc-600 px-2 py-0.5 rounded">
            {queue.queueId || "NOQUEUE"}
          </span>
          {queue.finalStatus && (
            <span style={{
              fontSize: 10, padding: "1px 8px", borderRadius: 4, fontWeight: 600,
              background: queue.finalStatus === "sent" ? "#f0fdf4" : queue.finalStatus === "bounced" ? "#fff1f2" : "#fffbeb",
              color: STATUS_COLOR[queue.finalStatus] ?? "#71717a",
            }}>{queue.finalStatus}</span>
          )}
        </div>
        {queue.from && <div className="text-xs text-zinc-400 mt-1 truncate">From: <span className="text-zinc-600">{queue.from}</span></div>}
        {queue.to   && <div className="text-xs text-zinc-400 truncate">To: <span className="text-zinc-600">{queue.to}</span></div>}
        <div className="text-xs text-zinc-400 mt-0.5">{queue.events.length} event{queue.events.length !== 1 ? "s" : ""}</div>
      </div>

      {/* Timeline */}
      <div className="flex-1 overflow-y-auto px-5 py-4">
        {queue.events.length === 0 ? (
          <EmptyState icon="search" title="No events found" compact />
        ) : (
          <div className="relative">
            <div className="absolute left-[6px] top-2 bottom-2 w-px bg-zinc-200" />
            <div className="space-y-4">
              {queue.events.map((ev, i) => (
                <div key={i} className="flex gap-3 relative">
                  <div className="shrink-0 mt-0.5 z-10 bg-white">
                    {STATUS_ICON[ev.kind === "delivery" ? ev.status : ev.kind] ?? <Clock size={14} style={{ color: "#a1a1aa", flexShrink: 0 }} />}
                  </div>
                  <div className="flex-1 min-w-0 pb-1">
                    <div className="flex items-center gap-2 mb-0.5">
                      <span className="text-xs font-medium" style={{ color: STATUS_COLOR[ev.status] ?? "#71717a" }}>
                        {ev.kind === "queued" ? "queued" : ev.kind === "removed" ? "removed" : ev.status || ev.kind}
                      </span>
                      <span className="text-xs text-zinc-400 tabular-nums">{fmtTime(ev.timestamp)}</span>
                      {i > 0 && ev.timestamp - queue.events[i - 1].timestamp > 0 && (
                        <span className="text-xs text-zinc-300 tabular-nums">{fmtDelay(ev.timestamp - queue.events[i - 1].timestamp)}</span>
                      )}
                      {ev.dsn && (
                        <span
                          className="text-xs font-mono bg-zinc-100 text-zinc-500 px-1.5 py-0 rounded cursor-help"
                          title={dsnExplain(ev.dsn)}
                        >{ev.dsn}</span>
                      )}
                    </div>
                    {ev.relay && (
                      <div className="text-xs text-zinc-400 truncate mb-0.5" title={ev.relay}>↳ {ev.relay}</div>
                    )}
                    {ev.message && (
                      <div className="text-xs text-zinc-500 leading-snug line-clamp-2" title={ev.message}>{ev.message}</div>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export function RetryTimelinePanel({ sinceHours, refreshKey }: Props) {
  const [queues, setQueues] = useState<string[]>([]);
  const [search, setSearch] = useState("");
  // journey = result of a search; selected = one queue from a multi-queue result
  const [journey, setJourney] = useState<JourneyResult | null>(null);
  const [selected, setSelected] = useState<JourneyQueue | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    (LogService.GetTopDeferredQueues(sinceHours, 50) as any)
      .then((r: any) => setQueues(r ?? []))
      .catch(() => setQueues([]));
    setJourney(null);
    setSelected(null);
    setSearch("");
  }, [sinceHours, refreshKey]);

  // Enter submits the search; empty search returns to the top-50 list.
  const runSearch = useCallback(() => {
    const q = search.trim();
    if (!q) {
      setJourney(null);
      setSelected(null);
      return;
    }
    setLoading(true);
    (LogService.GetJourney(q) as any)
      .then((r: any) => {
        const res: JourneyResult = r ?? { query: q, queues: [], truncated: false };
        setJourney(res);
        setSelected(res.queues.length === 1 ? res.queues[0] : null);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, [search]);

  const openQueue = useCallback((qid: string) => {
    setLoading(true);
    (LogService.GetJourney(qid) as any)
      .then((r: any) => {
        if (r?.queues?.length) setSelected(r.queues[0]);
        setLoading(false);
      })
      .catch(() => setLoading(false));
  }, []);

  // Single-queue search result → straight to the timeline
  if (journey && selected) return <JourneyTimelineView queue={selected} onBack={() => { setSelected(null); setJourney(null); setSearch(""); }} />;
  // One queue picked out of a multi-queue result
  if (selected) return <JourneyTimelineView queue={selected} onBack={() => setSelected(null)} />;

  const filtered = queues.filter(q => q.toLowerCase().includes(search.toLowerCase()));

  return (
    <div className="flex flex-col h-full">
      {/* Search */}
      <div className="shrink-0 px-4 py-3 border-b border-zinc-100">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 pointer-events-none" style={{ color: "#a1a1aa" }} />
          <Input
            placeholder="Queue ID or recipient…"
            value={search}
            onChange={e => setSearch(e.target.value)}
            onKeyDown={e => e.key === "Enter" && runSearch()}
            className="pl-10"
          />
        </div>
        <div className="text-xs text-zinc-400 mt-1.5">
          {journey ? (
            journey.queues.length === 0 ? "No matching messages" :
            `${journey.queues.length} matching queue${journey.queues.length !== 1 ? "s" : ""}${journey.truncated ? " (limited)" : ""} — click one`
          ) : (
            <>Top {queues.length} most-retried · search any queue ID or recipient</>
          )}
        </div>
      </div>

      {/* List */}
      <div className="flex-1 overflow-y-auto">
        {loading && <div className="flex items-center justify-center h-20 text-zinc-400 text-sm">Loading…</div>}
        {!loading && journey && journey.queues.map(q => (
          <button
            key={q.queueId || "noqueue"}
            onClick={() => setSelected(q)}
            className="w-full flex items-center justify-between gap-2 px-4 py-2.5 hover:bg-zinc-50 border-b border-zinc-50 text-left"
          >
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <span className="font-mono text-xs text-zinc-700">{q.queueId || "NOQUEUE"}</span>
                {q.finalStatus && (
                  <span className="text-[10px] font-medium" style={{ color: STATUS_COLOR[q.finalStatus] ?? "#71717a" }}>
                    {q.finalStatus}
                  </span>
                )}
              </div>
              {(q.to || q.from) && (
                <div className="text-[11px] text-zinc-400 truncate mt-0.5">{q.to || q.from}</div>
              )}
            </div>
            <ChevronRight size={12} className="text-zinc-300 shrink-0" />
          </button>
        ))}
        {!loading && !journey && filtered.map(qid => (
          <button
            key={qid}
            onClick={() => openQueue(qid)}
            className="w-full flex items-center justify-between px-4 py-2.5 hover:bg-zinc-50 border-b border-zinc-50 text-left"
          >
            <span className="font-mono text-xs text-zinc-700">{qid}</span>
            <ChevronRight size={12} className="text-zinc-300" />
          </button>
        ))}
      </div>
    </div>
  );
}
