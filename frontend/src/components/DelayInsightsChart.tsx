import { useState } from "react";
import {
  LineChart, Line, BarChart, Bar,
  XAxis, YAxis, CartesianGrid, Tooltip,
  ResponsiveContainer,
} from "recharts";
import { format } from "date-fns";

import { cn } from "@/lib/utils";

interface TimeSeriesPoint { timestamp: number; value: number }
interface DelayInsights {
  averageBeforeQmgr: number; averageInQmgr: number;
  averageConnSetup: number; averageTransmission: number;
  maxDelay: number; minDelay: number;
  averageDelay?: number;
  maxBeforeQmgr?: number; minBeforeQmgr?: number;
  maxInQmgr?: number; minInQmgr?: number;
  maxConnSetup?: number; minConnSetup?: number;
  maxTransmission?: number; minTransmission?: number;
  delayDistribution: Array<{ range: string; count: number; percentage: number }>;
  delayTrend: TimeSeriesPoint[];
  beforeQmgrTrend: TimeSeriesPoint[]; inQmgrTrend: TimeSeriesPoint[];
  connSetupTrend: TimeSeriesPoint[]; transmissionTrend: TimeSeriesPoint[];
}

const fmtDelay = (s: number): string => {
  if (s >= 3600) { const h = Math.floor(s / 3600); const m = Math.floor((s % 3600) / 60); return `${h}h ${m}m`; }
  if (s >= 60)   return `${Math.floor(s / 60)}m ${Math.round(s % 60)}s`;
  return `${s.toFixed(3)}s`;
};

const fmtY = (v: number) => v >= 1000 ? `${(v / 1000).toFixed(0)}k` : `${v}`;
const fmtYLog = (v: number) => {
  if (v >= 3600) return `${(v / 3600).toFixed(0)}h`;
  if (v >= 60)   return `${(v / 60).toFixed(0)}m`;
  if (v >= 1)    return `${v.toFixed(0)}s`;
  return `${v}s`;
};

const tooltipStyle = { backgroundColor: "#fff", border: "1px solid #e4e4e7", borderRadius: "8px", fontSize: 12 };
const gridColor = "#f4f4f5";
const tickColor = "#a1a1aa";

// Top-level stat card: Average / Min / Max in one row
function TopStatCard({ label, avg, min, max }: { label: string; avg: number; min: number; max: number }) {
  return (
    <div className="rounded-xl bg-white overflow-hidden" style={{border:"1px solid #e4e4e7"}}>
      <div className="p-5">
        <p className="text-sm text-zinc-400 mb-3">{label}</p>
        <div className="space-y-1.5">
          <div className="flex items-center justify-between">
            <span className="text-xs text-zinc-400">Avg:</span>
            <span className="text-sm font-bold text-zinc-900 tabular-nums">{fmtDelay(avg)}</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-xs text-zinc-400">Min:</span>
            <span className="text-sm font-semibold text-foreground tabular-nums">{fmtDelay(min)}</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-xs text-zinc-400">Max:</span>
            <span className="text-sm font-semibold text-foreground tabular-nums">{fmtDelay(max)}</span>
          </div>
        </div>
      </div>
    </div>
  );
}

// Wide stat card for overall avg/min/max
function SummaryCard({ label, value, color }: { label: string; value: number; color: "neutral" | "green" | "red" }) {
  return (
    <div className="rounded-xl bg-white overflow-hidden" style={{border:"1px solid #e4e4e7"}}>
      <div className="p-5 flex flex-col gap-1">
        <p className="text-sm text-zinc-400">{label}</p>
        <p className={cn(
          "text-2xl font-bold tabular-nums",
          "text-foreground"
        )}>
          {fmtDelay(value)}
        </p>
      </div>
    </div>
  );
}

// Transmission wide card — centered avg/min/max
function TransmissionCard({ avg, min, max }: { avg: number; min: number; max: number }) {
  return (
    <div className="rounded-xl bg-white overflow-hidden" style={{border:"1px solid #e4e4e7"}}>
      <div className="p-5">
        <p className="text-sm text-zinc-400 mb-4">Transmission</p>
        <div className="flex items-end justify-center gap-12">
          <div className="text-center">
            <p className="text-xs text-zinc-400 mb-1">Average</p>
            <p className="text-xl font-bold text-zinc-900 tabular-nums">{fmtDelay(avg)}</p>
          </div>
          <div className="text-center">
            <p className="text-xs text-zinc-400 mb-1">Minimum</p>
            <p className="text-xl font-bold text-foreground tabular-nums">{fmtDelay(min)}</p>
          </div>
          <div className="text-center">
            <p className="text-xs text-zinc-400 mb-1">Maximum</p>
            <p className="text-xl font-bold text-foreground tabular-nums">{fmtDelay(max)}</p>
          </div>
        </div>
      </div>
    </div>
  );
}

export default function DelayInsightsChart({ delayInsights }: { delayInsights: DelayInsights }) {
  const d = delayInsights;

  // Build trend data — keep ts as number, series values as number|null
  type TrendRow = { ts: number; label: string; beforeQmgr: number; inQmgr: number; connSetup: number; transmission: number; };
  const trendMap = new Map<number, TrendRow>();
  const addSeries = (pts: TimeSeriesPoint[], key: keyof Omit<TrendRow, 'ts'|'label'>) =>
    pts.forEach(({ timestamp, value }) => {
      if (!trendMap.has(timestamp)) trendMap.set(timestamp, { ts: timestamp, label: "", beforeQmgr: 0, inQmgr: 0, connSetup: 0, transmission: 0 });
      // Keep raw values; dual axis handles the scale difference
      trendMap.get(timestamp)![key] = +value.toFixed(3);
    });
  addSeries(d.beforeQmgrTrend,    "beforeQmgr");
  addSeries(d.inQmgrTrend,        "inQmgr");
  addSeries(d.connSetupTrend,     "connSetup");
  addSeries(d.transmissionTrend,  "transmission");

  const trendData = Array.from(trendMap.values())
    .sort((a, b) => a.ts - b.ts)
    .map(r => ({ ...r, label: format(new Date(r.ts), "HH:mm") }));

  const avgDelay = d.averageDelay ?? d.averageBeforeQmgr;
  const minDelay = d.minDelay === 1e308 ? 0 : d.minDelay;

  return (
    <div className="space-y-4">

      {/* Row 1: overall summary */}
      <div className="grid grid-cols-3 gap-4">
        <SummaryCard label="Average Delay" value={avgDelay}  color="neutral" />
        <SummaryCard label="Min Delay"     value={minDelay}  color="green"   />
        <SummaryCard label="Max Delay"     value={d.maxDelay} color="red"    />
      </div>

      {/* Row 2: per-stage breakdown */}
      <div className="grid grid-cols-3 gap-4">
        <TopStatCard
          label="Before Queue Mgr"
          avg={d.averageBeforeQmgr}
          min={d.minBeforeQmgr ?? 0}
          max={d.maxBeforeQmgr ?? 0}
        />
        <TopStatCard
          label="In Queue Mgr"
          avg={d.averageInQmgr}
          min={d.minInQmgr ?? 0}
          max={d.maxInQmgr ?? 0}
        />
        <TopStatCard
          label="Connection Setup"
          avg={d.averageConnSetup}
          min={d.minConnSetup ?? 0}
          max={d.maxConnSetup ?? 0}
        />
      </div>

      {/* Row 3: transmission wide card */}
      <TransmissionCard
        avg={d.averageTransmission}
        min={d.minTransmission ?? 0}
        max={d.maxTransmission ?? 0}
      />

      {/* Row 4: distribution bar chart */}
      {d.delayDistribution?.length > 0 && (
        <div className="rounded-xl bg-white overflow-hidden" style={{border:"1px solid #e4e4e7"}}>
          <div className="px-5 py-3.5 border-b border-zinc-100">
            <p className="text-sm font-semibold text-zinc-900">Delay Distribution</p>
            <p className="text-xs text-zinc-400 mt-0.5">Message count per delay bucket</p>
          </div>
          <div className="pt-4">
            <ResponsiveContainer width="100%" height={200} minWidth={300}>
              <BarChart data={d.delayDistribution} margin={{ top: 0, right: 8, left: 10, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" stroke={gridColor} vertical={false} />
                <XAxis dataKey="range" tick={{ fill: tickColor, fontSize: 11 }} axisLine={false} tickLine={false} />
                <YAxis tick={{ fill: tickColor, fontSize: 11 }} axisLine={false} tickLine={false} tickFormatter={fmtY} />
                <Tooltip contentStyle={tooltipStyle} cursor={{ fill: "#f9f9f9" }} />
                <Bar dataKey="count" fill="#10b981" radius={[3, 3, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      )}

      {/* Row 5: trend line chart with toggleable series */}
      {trendData.length > 0 && (
        <TrendChart trendData={trendData} />
      )}

    </div>
  );
}

// ── Trend chart with per-series toggle ───────────────────────────────────────
const SERIES = [
  { key: "beforeQmgr",   name: "Before Qmgr",  color: "#10b981" },
  { key: "inQmgr",       name: "In Qmgr",       color: "#3b82f6" },
  { key: "connSetup",    name: "Conn Setup",    color: "#8b5cf6" },
  { key: "transmission", name: "Transmission",  color: "#f97316" },
] as const;

function TrendChart({ trendData }: { trendData: Record<string, any>[] }) {
  const [hidden, setHidden] = useState<Set<string>>(new Set());

  const toggle = (key: string) =>
    setHidden(prev => {
      const next = new Set(prev);
      next.has(key) ? next.delete(key) : next.add(key);
      return next;
    });

  return (
    <div className="rounded-xl bg-white overflow-hidden" style={{border:"1px solid #e4e4e7"}}>
      <div className="px-5 py-3.5 border-b border-zinc-100 flex items-center justify-between gap-4">
        <div>
          <p className="text-sm font-semibold text-zinc-900">Delay Breakdown Over Time</p>
          <p className="text-xs text-zinc-400 mt-0.5">Click a series to show/hide</p>
        </div>
        {/* Custom legend toggles */}
        <div className="flex items-center gap-2 flex-wrap justify-end">
          {SERIES.map(s => {
            const active = !hidden.has(s.key);
            return (
              <button
                key={s.key}
                onClick={() => toggle(s.key)}
                className={cn(
                  "flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium border transition-all",
                  active
                    ? "border-transparent text-white"
                    : "border-zinc-200 text-zinc-400 bg-white"
                )}
                style={active ? { backgroundColor: s.color, borderColor: s.color } : {}}
              >
                <span className="w-2 h-2 rounded-full shrink-0"
                  style={{ backgroundColor: active ? "rgba(255,255,255,0.7)" : "#d4d4d8" }} />
                {s.name}
              </button>
            );
          })}
        </div>
      </div>
      <div className="pt-4">
        <ResponsiveContainer width="100%" height={300} minWidth={300}>
          <LineChart data={trendData} margin={{ top: 4, right: 8, left: 10, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke={gridColor} vertical={false} />
            <XAxis dataKey="label" tick={{ fill: tickColor, fontSize: 11 }} axisLine={false} tickLine={false} interval="preserveStartEnd" />
            {/* Left axis: Before Qmgr (large values) */}
            <YAxis yAxisId="left" tick={{ fill: "#10b981", fontSize: 11 }} axisLine={false} tickLine={false} tickFormatter={fmtYLog} width={44} />
            {/* Right axis: In Qmgr, Conn Setup, Transmission (small values) */}
            <YAxis yAxisId="right" orientation="right" tick={{ fill: tickColor, fontSize: 11 }} axisLine={false} tickLine={false} tickFormatter={fmtYLog} width={32} />
            <Tooltip contentStyle={tooltipStyle} formatter={(v: any, name: any) => [`${v}s`, name]} />
            {SERIES.map(s => (
              <Line
                key={s.key}
                yAxisId={s.key === "beforeQmgr" ? "left" : "right"}
                type="monotone"
                dataKey={s.key}
                name={s.name}
                stroke={s.color}
                dot={false}
                strokeWidth={hidden.has(s.key) ? 0 : 2}
                hide={hidden.has(s.key)}
                connectNulls={false}
              />
            ))}
          </LineChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
