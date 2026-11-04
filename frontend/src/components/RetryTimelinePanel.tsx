import { useEffect, useState, useCallback } from "react";
import { Search, ChevronRight, Clock, CheckCircle2, XCircle, RefreshCw } from "lucide-react";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";
import { EmptyState } from "./EmptyState";

interface RetryAttempt {
  timestamp: number;
  status: string;
  dsn: string;
  relay: string;
  message: string;
  delayMs: number;
}

interface RetryTimeline {
  queueId: string;
  from: string;
  to: string;
  attempts: RetryAttempt[];
  finalStatus: string;
}

interface Props {
  sinceHours: number;
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
  deferred: <RefreshCw   size={14} style={{ color: "#eab308", flexShrink: 0 }} />,
};

const STATUS_COLOR: Record<string, string> = {
  sent: "#10b981", bounced: "#f43f5e", deferred: "#eab308",
};

function TimelineView({ timeline, onBack }: { timeline: RetryTimeline; onBack: () => void }) {
  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="shrink-0 px-5 py-3 border-b border-zinc-100">
        <button onClick={onBack} className="text-xs text-zinc-400 hover:text-zinc-600 mb-2 flex items-center gap-1">
          ← Back
        </button>
        <div className="flex items-center gap-2 flex-wrap">
          <span className="font-mono text-xs bg-zinc-100 text-zinc-600 px-2 py-0.5 rounded">{timeline.queueId}</span>
          <span style={{
            fontSize: 10, padding: "1px 8px", borderRadius: 4, fontWeight: 600,
            background: timeline.finalStatus === "sent" ? "#f0fdf4" : timeline.finalStatus === "bounced" ? "#fff1f2" : "#fffbeb",
            color: STATUS_COLOR[timeline.finalStatus] ?? "#71717a",
          }}>{timeline.finalStatus || "active"}</span>
        </div>
        {timeline.from && <div className="text-xs text-zinc-400 mt-1 truncate">From: <span className="text-zinc-600">{timeline.from}</span></div>}
        {timeline.to   && <div className="text-xs text-zinc-400 truncate">To: <span className="text-zinc-600">{timeline.to}</span></div>}
        <div className="text-xs text-zinc-400 mt-0.5">{timeline.attempts.length} attempt{timeline.attempts.length !== 1 ? "s" : ""}</div>
      </div>

      {/* Timeline */}
      <div className="flex-1 overflow-y-auto px-5 py-4">
        {timeline.attempts.length === 0 ? (
          <EmptyState icon="search" title="No delivery attempts found" compact />
        ) : (
          <div className="relative">
            {/* Vertical line */}
            <div className="absolute left-[6px] top-2 bottom-2 w-px bg-zinc-200" />
            <div className="space-y-4">
              {timeline.attempts.map((a, i) => (
                <div key={i} className="flex gap-3 relative">
                  {/* Icon */}
                  <div className="shrink-0 mt-0.5 z-10 bg-white">
                    {STATUS_ICON[a.status] ?? <Clock size={14} style={{ color: "#a1a1aa", flexShrink: 0 }} />}
                  </div>
                  {/* Content */}
                  <div className="flex-1 min-w-0 pb-1">
                    <div className="flex items-center gap-2 mb-0.5">
                      <span className="text-xs font-medium" style={{ color: STATUS_COLOR[a.status] ?? "#71717a" }}>
                        {a.status}
                      </span>
                      <span className="text-xs text-zinc-400 tabular-nums">{fmtTime(a.timestamp)}</span>
                      {a.delayMs > 0 && (
                        <span className="text-xs text-zinc-300 tabular-nums">{fmtDelay(a.delayMs)}</span>
                      )}
                      {a.dsn && (
                        <span className="text-xs font-mono bg-zinc-100 text-zinc-500 px-1.5 py-0 rounded">{a.dsn}</span>
                      )}
                    </div>
                    {a.relay && (
                      <div className="text-xs text-zinc-400 truncate mb-0.5" title={a.relay}>↳ {a.relay}</div>
                    )}
                    {a.message && (
                      <div className="text-xs text-zinc-500 leading-snug line-clamp-2" title={a.message}>{a.message}</div>
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

export function RetryTimelinePanel({ sinceHours }: Props) {
  const [queues, setQueues] = useState<string[]>([]);
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<RetryTimeline | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    (LogService.GetTopDeferredQueues(sinceHours, 50) as any)
      .then((r: any) => setQueues(r ?? []))
      .catch(() => setQueues([]));
    setSelected(null);
    setSearch("");
  }, [sinceHours]);

  const open = useCallback((queueId: string) => {
    setLoading(true);
    (LogService.GetRetryTimeline(queueId) as any)
      .then((r: any) => { setSelected(r); setLoading(false); })
      .catch(() => setLoading(false));
  }, []);

  if (selected) return <TimelineView timeline={selected} onBack={() => setSelected(null)} />;

  const filtered = queues.filter(q => q.toLowerCase().includes(search.toLowerCase()));

  return (
    <div className="flex flex-col h-full">
      {/* Search */}
      <div className="shrink-0 px-4 py-3 border-b border-zinc-100">
        <div className="relative">
          <Search size={12} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-zinc-400" />
          <input
            className="w-full pl-7 pr-3 py-1.5 text-xs border border-zinc-200 rounded-lg bg-white focus:outline-none focus:ring-1 focus:ring-zinc-300"
            placeholder="Search queue ID…"
            value={search}
            onChange={e => setSearch(e.target.value)}
          />
        </div>
        <div className="text-xs text-zinc-400 mt-1.5">Top {queues.length} most-retried queues</div>
      </div>

      {/* List */}
      <div className="flex-1 overflow-y-auto">
        {loading && <div className="flex items-center justify-center h-20 text-zinc-400 text-sm">Loading…</div>}
        {!loading && filtered.length === 0 && (
          <div className="p-4"><EmptyState icon="search" title="No queues found" compact /></div>
        )}
        {!loading && filtered.map(qid => (
          <button
            key={qid}
            onClick={() => open(qid)}
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
