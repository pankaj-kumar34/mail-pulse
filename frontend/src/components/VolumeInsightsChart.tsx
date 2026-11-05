import { useState, useEffect, useRef, useCallback } from "react";
import {
  ComposedChart, Bar, XAxis, YAxis, CartesianGrid,
  Tooltip, ResponsiveContainer, Legend,
} from "recharts";
import { RecipientDrilldownPanel } from "./RecipientDrilldownPanel";
import { ReasonDrilldownPanel } from "./ReasonDrilldownPanel";
import { QueueAgeChart } from "./QueueAgeChart";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";
import { EmptyState } from "./EmptyState";
import { HourlyHeatmap } from "./HourlyHeatmap";

interface HourlyVolume { hour: number; sent: number; deferred: number; bounced: number; }
interface Reason       { reason: string; count: number; percent: number; }
interface DomainDelivery { domain: string; sent: number; deferred: number; bounced: number; total: number; failRate: number; }
interface SenderStat   { address: string; sent: number; deferred: number; bounced: number; total: number; failRate: number; }
interface VolumeInsights {
  hourly: HourlyVolume[];
  deferReasons: Reason[];
  bounceReasons: Reason[];
  failingDomains: DomainDelivery[];
}

const C = {
  sent:     "#10b981",  // emerald — healthy delivery
  deferred: "#eab308",  // yellow-500 — temporary failure
  bounced:  "#f43f5e",  // rose  — permanent failure
  grid:     "#f4f4f5",
  tick:     "#a1a1aa",
  border:   "#e4e4e7",
};

const tooltipStyle = {
  backgroundColor: "#fff", border: `1px solid ${C.border}`,
  borderRadius: "8px", fontSize: 12, boxShadow: "none",
};

// Curated sequential palette for deferred reasons (yellow family)
const DEFER_COLORS = [
  "#eab308", "#facc15", "#fde047", "#ca8a04", "#fef08a", "#a16207",
];

// Curated sequential palette for bounce reasons (rose/red family)
const BOUNCE_COLORS = [
  "#f43f5e", "#fb7185", "#e11d48", "#fda4af", "#be123c", "#fecdd3",
];

function fmtHour(ts: number) {
  return new Date(ts * 1000).toLocaleTimeString("en-GB", { hour: "2-digit", minute: "2-digit", hour12: false });
}

// Card wrapper matching Qmgr style
function Card({ title, subtitle, children, className = "", maxHeight }: {
  title: string;
  subtitle?: string;
  children: React.ReactNode;
  className?: string;
  maxHeight?: number;
}) {
  return (
    <div className={`rounded-xl border border-zinc-100 bg-white flex flex-col ${className}`} style={{ position: "relative" }}>
      <div className="px-5 py-3.5 border-b border-zinc-100 shrink-0">
        <h3 className="text-sm font-semibold text-zinc-800">{title}</h3>
        {subtitle && <p className="text-xs mt-0.5 text-zinc-400">{subtitle}</p>}
      </div>
      <div className="p-4" style={maxHeight ? { overflowY: "auto", maxHeight } : { flex: 1, minHeight: 0 }}>
        {children}
      </div>
    </div>
  );
}

function ReasonList({ items, colors, onDrill }: { items: Reason[]; colors: string[]; onDrill?: (reason: string) => void }) {
  if (!items?.length) return <EmptyState icon="check" title="None" subtitle="No events in this time range" compact />;
  return (
    <div className="space-y-2.5">
      {items.map((r, i) => (
        <div key={r.reason} onClick={() => onDrill?.(r.reason)}
          style={{ cursor: onDrill ? "pointer" : "default" }}
          className={onDrill ? "hover:opacity-70 transition-opacity" : ""}>
          <div className="flex justify-between text-xs mb-1">
            <div className="flex items-center gap-1.5 min-w-0">
              <div className="w-2 h-2 rounded-full shrink-0"
                style={{ background: colors[i % colors.length] }} />
              <span className="text-zinc-700 font-medium truncate underline decoration-dotted underline-offset-2">{r.reason}</span>
            </div>
            <span className="text-zinc-400 tabular-nums shrink-0 ml-2">
              {r.count.toLocaleString()}
              <span className="text-zinc-300 ml-1">({r.percent.toFixed(0)}%)</span>
            </span>
          </div>
          <div className="h-1.5 rounded-full overflow-hidden" style={{ background: C.grid }}>
            <div className="h-full rounded-full"
              style={{ width: `${r.percent}%`, background: colors[i % colors.length] }} />
          </div>
        </div>
      ))}
    </div>
  );
}

function FailBar({ failRate, failCount, maxFailCount }: { failRate: number; failCount: number; maxFailCount: number }) {
  const barPct    = maxFailCount > 0 ? (failCount / maxFailCount) * 100 : 0;
  // All bars here represent failures — use rose/amber only, never green
  const color     = failRate >= 60 ? "#f43f5e" : "#eab308";
  const textColor = failRate >= 60 ? "#e11d48" : "#a16207";
  return (
    <div className="flex items-center gap-2">
      <div className="flex-1 h-1.5 rounded-full overflow-hidden" style={{ background: C.grid }}>
        <div className="h-full rounded-full" style={{ width: `${barPct}%`, background: color }} />
      </div>
      <span className="text-xs tabular-nums whitespace-nowrap w-20 text-right shrink-0"
        style={{ color: textColor }}>
        {failRate.toFixed(1)}% fail
      </span>
    </div>
  );
}

export function VolumeInsightsChart({ data, sinceHours }: { data: VolumeInsights | null; sinceHours: number }) {
  const [drillDomain, setDrillDomain]   = useState<string | null>(null);
  const [drillReason, setDrillReason]   = useState<{ reason: string; status: "deferred" | "bounced" } | null>(null);
  const [drillSender, setDrillSender]   = useState<string | null>(null);
  const [senderStats, setSenderStats]   = useState<SenderStat[]>([]);
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => { scrollRef.current?.scrollTo({ top: 0 }); }, [sinceHours]);

  useEffect(() => {
    LogService.GetSenderStats(sinceHours)
      .then((r: any) => setSenderStats(r ?? []))
      .catch(() => setSenderStats([]));
  }, [sinceHours, data]);

  const onDrillDefer  = useCallback((reason: string) => setDrillReason({ reason, status: "deferred" }), []);
  const onDrillBounce = useCallback((reason: string) => setDrillReason({ reason, status: "bounced" }), []);
  const closeReason   = useCallback(() => setDrillReason(null), []);
  const closeSender   = useCallback(() => setDrillSender(null), []);

  if (!data) return (
    <div className="flex-1 flex items-center justify-center"><EmptyState icon="chart" title="No data" subtitle="Load a log file to see volume insights" /></div>
  );

  const deferColors  = DEFER_COLORS;
  const bounceColors = BOUNCE_COLORS;

  return (
    <div className="flex flex-col h-full" style={{ position: "relative" }}>

      <div ref={scrollRef} className="flex flex-col flex-1 overflow-auto px-5 pt-3 pb-4 gap-4">

        {/* ── Row 1: Hourly volume chart ── */}
        <Card title="Hourly Volume" subtitle="Sent · deferred · bounced per hour" className="shrink-0">
          <div style={{ height: 180 }}>
            <ResponsiveContainer width="100%" height="100%">
              <ComposedChart data={data.hourly} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke={C.grid} />
                <XAxis dataKey="hour" tickFormatter={fmtHour}
                  tick={{ fontSize: 11, fill: C.tick }} axisLine={false} tickLine={false} />
                <YAxis tick={{ fontSize: 11, fill: C.tick }} width={36}
                  axisLine={false} tickLine={false} />
                <Tooltip contentStyle={tooltipStyle}
                  labelFormatter={v => `Hour: ${fmtHour(Number(v))}`}
                  cursor={{ fill: "#f4f4f5" }} />
                <Legend wrapperStyle={{ fontSize: 12, color: "#71717a" }} />
                <Bar dataKey="sent"     name="Sent"     fill={C.sent}     radius={[0,0,0,0]} maxBarSize={28} stackId="vol" />
                <Bar dataKey="deferred" name="Deferred" fill={C.deferred} radius={[0,0,0,0]} maxBarSize={28} stackId="vol" />
                <Bar dataKey="bounced"  name="Bounced"  fill={C.bounced}  radius={[2,2,0,0]} maxBarSize={28} stackId="vol" />
              </ComposedChart>
            </ResponsiveContainer>
          </div>
        </Card>

        {/* ── Row 2: Queue age distribution ── */}
        <Card title="Queue Age Distribution" subtitle="How long messages spend in the queue before final delivery" className="shrink-0">
          <QueueAgeChart sinceHours={sinceHours} />
        </Card>

        {/* ── Row 3: Two-column — Deferred + Bounce reasons ── */}
        <div className="grid grid-cols-2 gap-4 shrink-0">

          {/* Deferred reasons */}
          <Card title="Deferred Reasons" subtitle="Click a reason to see messages" maxHeight={280}>
            {drillReason && drillReason.status === "deferred" && (
              <ReasonDrilldownPanel reason={drillReason.reason} status="deferred" sinceHours={sinceHours} onClose={closeReason} />
            )}
            <ReasonList items={data.deferReasons} colors={deferColors} onDrill={onDrillDefer} />
          </Card>

          {/* Bounce reasons */}
          <Card title="Bounce Reasons" subtitle="Click a reason to see messages" maxHeight={280}>
            {drillReason && drillReason.status === "bounced" && (
              <ReasonDrilldownPanel reason={drillReason.reason} status="bounced" sinceHours={sinceHours} onClose={closeReason} />
            )}
            <ReasonList items={data.bounceReasons} colors={bounceColors} onDrill={onDrillBounce} />
          </Card>

        </div>

        {/* ── Row 4: Two-column — Top failing destinations + senders ── */}
        <div className="grid grid-cols-2 gap-4 shrink-0">

          {/* Top failing destinations */}
          <Card title="Top Failing Destinations" subtitle="Click a domain to see recipient breakdown" maxHeight={280}>
            {drillDomain && (
              <RecipientDrilldownPanel
                domain={drillDomain}
                sinceHours={sinceHours}
                onClose={() => setDrillDomain(null)}
              />
            )}
            {!data.failingDomains?.length ? (
              <EmptyState icon="check" title="No failing destinations" subtitle="All domains delivered successfully" compact />
            ) : (
              <div className="space-y-2.5">
                {(() => {
                  const maxFailCount = Math.max(...data.failingDomains.map(d => d.deferred + d.bounced));
                  return data.failingDomains.slice(0, 10).map(d => (
                    <div key={d.domain} onClick={() => setDrillDomain(d.domain)}
                      className="cursor-pointer hover:opacity-70 transition-opacity">
                      <div className="flex justify-between text-xs mb-1">
                        <span className="text-zinc-700 font-medium truncate max-w-[200px] underline decoration-dotted underline-offset-2">{d.domain}</span>
                        <span className="text-zinc-400 tabular-nums shrink-0 ml-2">
                          {(d.deferred + d.bounced).toLocaleString()} failed
                        </span>
                      </div>
                      <FailBar failRate={d.failRate} failCount={d.deferred + d.bounced} maxFailCount={maxFailCount} />
                    </div>
                  ));
                })()}
              </div>
            )}
          </Card>

          {/* Top failing senders */}
          <Card title="Top Failing Senders" subtitle="Click a sender to see their messages" maxHeight={280}>
            {drillSender && (
              <ReasonDrilldownPanel
                reason={drillSender} status="" sinceHours={sinceHours} onClose={closeSender}
                title={drillSender}
                fetchFn={(h) => LogService.GetMessagesBySender(drillSender, "failed", h) as any}
              />
            )}
            {!senderStats.length ? (
              <EmptyState icon="check" title="No failing senders" subtitle="All senders delivered successfully" compact />
            ) : (
              <div className="space-y-2.5">
                {(() => {
                  const maxFail = Math.max(...senderStats.slice(0, 10).map(s => s.deferred + s.bounced));
                  return senderStats.slice(0, 10).map(s => (
                    <div key={s.address} onClick={() => setDrillSender(s.address)}
                      className="cursor-pointer hover:opacity-70 transition-opacity">
                      <div className="flex justify-between text-xs mb-1">
                        <span className="text-zinc-700 font-medium truncate max-w-[240px] underline decoration-dotted underline-offset-2">{s.address}</span>
                        <span className="text-zinc-400 tabular-nums shrink-0 ml-2">
                          {(s.deferred + s.bounced).toLocaleString()} failed
                        </span>
                      </div>
                      <FailBar failRate={s.failRate} failCount={s.deferred + s.bounced} maxFailCount={maxFail} />
                    </div>
                  ));
                })()}
              </div>
            )}
          </Card>

        </div>

        {/* ── Row 5: Hourly heatmap ── */}
        <Card title="Hourly Activity Heatmap" subtitle="Volume by hour · color = fail rate" className="shrink-0">
          <HourlyHeatmap sinceHours={sinceHours} />
        </Card>

      </div>
    </div>
  );
}
