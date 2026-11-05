import { useState } from "react";
import {
  XAxis, YAxis, CartesianGrid, Tooltip,
  ResponsiveContainer, BarChart, Bar, Area, AreaChart,
} from "recharts";
import { format } from "date-fns";
import { EmptyState } from "./EmptyState";
import { TrendingUp, TrendingDown, Activity, Minus } from "lucide-react";

interface TimeSeriesPoint { timestamp: number; value: number }

const tooltipStyle = {
  backgroundColor: "#fff",
  border: "1px solid #e4e4e7",
  borderRadius: "8px",
  color: "#09090b",
  fontSize: 12,
};

export function QmgrActivityChart({
  activeData,
  removedData,
}: {
  activeData: TimeSeriesPoint[];
  removedData: TimeSeriesPoint[];
}) {
  const [showActive,  setShowActive]  = useState(true);
  const [showRemoved, setShowRemoved] = useState(true);

  const map = new Map<number, Record<string, number>>();
  activeData.forEach(({ timestamp, value }) => {
    if (!map.has(timestamp)) map.set(timestamp, { timestamp });
    map.get(timestamp)!.active = value;
  });
  removedData.forEach(({ timestamp, value }) => {
    if (!map.has(timestamp)) map.set(timestamp, { timestamp });
    map.get(timestamp)!.removed = value;
  });

  type RawPoint = { timestamp: number; active?: number; removed?: number };
  // Backend bucketed to 5-min intervals — use directly, no re-bucketing needed
  const data = (Array.from(map.values()) as RawPoint[])
    .sort((a, b) => a.timestamp - b.timestamp)
    .map(d => ({
      timestamp: d.timestamp,
      active:    d.active  ?? 0,
      removed:   d.removed ?? 0,
      label:     format(new Date(d.timestamp), "HH:mm"),
    }));

  if (data.length === 0) return (
    <div className="h-full flex items-center justify-center"><EmptyState icon="chart" title="No qmgr data" subtitle="Load a log file to see queue activity" fill /></div>
  );

  const totalActive  = data.reduce((s, p) => s + p.active,  0);
  const totalRemoved = data.reduce((s, p) => s + p.removed, 0);
  const peakActive   = data.length ? data.reduce((m, p) => Math.max(m, p.active),  0) : 0;
  const peakRemoved  = data.length ? data.reduce((m, p) => Math.max(m, p.removed), 0) : 0;
  const avgActive    = data.length ? (totalActive  / data.length).toFixed(1) : "0";
  const avgRemoved   = data.length ? (totalRemoved / data.length).toFixed(1) : "0";

  // Hourly bar data — bucket by local hour, sort chronologically
  const hourMap = new Map<number, { label: string; active: number; removed: number }>();
  data.forEach(d => {
    const date = new Date(d.timestamp);
    // floor to start of local hour as epoch ms
    const hourEpoch = new Date(date.getFullYear(), date.getMonth(), date.getDate(), date.getHours()).getTime();
    if (!hourMap.has(hourEpoch)) hourMap.set(hourEpoch, { label: format(date, "HH:00"), active: 0, removed: 0 });
    const b = hourMap.get(hourEpoch)!;
    b.active  += d.active;
    b.removed += d.removed;
  });
  const hourlyData = Array.from(hourMap.entries())
    .sort((a, b) => a[0] - b[0])
    .map(([, vals]) => vals);

  const statCards = [
    { label: "Total Active",    value: totalActive.toLocaleString(),  icon: Activity,     color: "text-blue-600",    bg: "bg-blue-50",     border: "border-blue-100"    },
    { label: "Total Removed",   value: totalRemoved.toLocaleString(), icon: TrendingDown, color: "text-emerald-600", bg: "bg-emerald-50",  border: "border-emerald-100" },
    { label: "Peak Active",     value: peakActive.toString(),         icon: TrendingUp,   color: "text-amber-600",   bg: "bg-amber-50",    border: "border-amber-100"   },
    { label: "Peak Removed",    value: peakRemoved.toString(),        icon: TrendingUp,   color: "text-purple-600",  bg: "bg-purple-50",   border: "border-purple-100"  },
    { label: "Avg Active/5min",  value: avgActive,                     icon: Minus,        color: "text-zinc-600",    bg: "bg-zinc-50",     border: "border-zinc-200"    },
    { label: "Avg Removed/5min", value: avgRemoved,                    icon: Minus,        color: "text-zinc-600",    bg: "bg-zinc-50",     border: "border-zinc-200"    },
  ];

  return (
    <div className="space-y-4 h-full flex flex-col">

      {/* Stat cards */}
      <div className="grid grid-cols-6 gap-3 shrink-0">
        {statCards.map(s => (
          <div key={s.label} className={`rounded-xl border ${s.border} bg-white p-3.5 flex flex-col gap-2`}>
            <div className="flex items-center justify-between">
              <span className="text-xs font-medium text-zinc-400 uppercase tracking-wide leading-tight">{s.label}</span>
              <div className={`p-1 rounded-lg ${s.bg}`}>
                <s.icon className={`w-3.5 h-3.5 ${s.color}`} />
              </div>
            </div>
            <span className={`text-xl font-bold tabular-nums ${s.color}`}>{s.value}</span>
          </div>
        ))}
      </div>

      {/* Main area chart */}
      <div className="flex-1 min-h-0 rounded-xl border border-zinc-100 bg-white flex flex-col">
        <div className="px-5 py-3.5 border-b border-zinc-100 shrink-0 flex items-center justify-between">
          <div>
            <h3 className="text-sm font-semibold text-zinc-800">Activity Over Time</h3>
            <p className="text-xs mt-0.5 text-zinc-400">Active vs removed messages per 5 minutes</p>
          </div>
          {/* Toggle pills */}
          <div className="flex items-center gap-2">
            <button
              onClick={() => setShowActive(v => !v)}
              className={`flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium border transition-all ${
                showActive
                  ? "bg-blue-50 border-blue-200 text-blue-700"
                  : "bg-white border-zinc-200 text-zinc-400"
              }`}
            >
              <span className={`w-2 h-2 rounded-full ${showActive ? "bg-blue-500" : "bg-zinc-300"}`} />
              Active
            </button>
            <button
              onClick={() => setShowRemoved(v => !v)}
              className={`flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-medium border transition-all ${
                showRemoved
                  ? "bg-emerald-50 border-emerald-200 text-emerald-700"
                  : "bg-white border-zinc-200 text-zinc-400"
              }`}
            >
              <span className={`w-2 h-2 rounded-full ${showRemoved ? "bg-emerald-500" : "bg-zinc-300"}`} />
              Removed
            </button>
          </div>
        </div>
        <div className="flex-1 min-h-0 p-4">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data} margin={{ top: 4, right: 8, left: 10, bottom: 20 }}>
              <defs>
                <linearGradient id="gradActive" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%"  stopColor="#3b82f6" stopOpacity={0.12} />
                  <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="gradRemoved" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%"  stopColor="#10b981" stopOpacity={0.12} />
                  <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="#f4f4f5" vertical={false} />
              <XAxis dataKey="label" tick={{ fill: "#a1a1aa", fontSize: 11 }} axisLine={false} tickLine={false} interval="preserveStartEnd" />
              <YAxis tick={{ fill: "#a1a1aa", fontSize: 11 }} axisLine={false} tickLine={false} tickFormatter={(v) => v >= 1000 ? `${(v/1000).toFixed(0)}k` : `${v}`} domain={[0, "auto"]} />
              <Tooltip contentStyle={tooltipStyle} cursor={{ stroke: "#e4e4e7" }} />
              {showActive  && <Area type="monotone" dataKey="active"  name="Active"  stroke="#3b82f6" strokeWidth={2} fill="url(#gradActive)"  dot={false} connectNulls={true} isAnimationActive={false} />}
              {showRemoved && <Area type="monotone" dataKey="removed" name="Removed" stroke="#10b981" strokeWidth={2} fill="url(#gradRemoved)" dot={false} connectNulls={true} isAnimationActive={false} />}
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Hourly breakdown bar chart */}
      {hourlyData.length > 1 && (
        <div className="rounded-xl border border-zinc-100 bg-white shrink-0">
          <div className="px-5 py-3.5 border-b border-zinc-100">
            <h3 className="text-sm font-semibold text-zinc-800">Hourly Breakdown</h3>
            <p className="text-xs mt-0.5 text-zinc-400">Total messages active/removed per hour</p>
          </div>
          <div className="p-4">
            <ResponsiveContainer width="100%" height={160}>
              <BarChart data={hourlyData} margin={{ top: 4, right: 8, left: 10, bottom: 0 }} barCategoryGap="30%">
                <CartesianGrid strokeDasharray="3 3" stroke="#f4f4f5" vertical={false} />
                <XAxis dataKey="label" tick={{ fill: "#a1a1aa", fontSize: 11 }} axisLine={false} tickLine={false} />
                <YAxis tick={{ fill: "#a1a1aa", fontSize: 11 }} axisLine={false} tickLine={false} tickFormatter={(v) => v >= 1000 ? `${(v/1000).toFixed(0)}k` : `${v}`} />
                <Tooltip contentStyle={tooltipStyle} cursor={{ fill: "#f9f9f9" }} />
                <Bar dataKey="active"  name="Active"  fill="#3b82f6" radius={[3, 3, 0, 0]} />
                <Bar dataKey="removed" name="Removed" fill="#10b981" radius={[3, 3, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      )}

    </div>
  );
}
