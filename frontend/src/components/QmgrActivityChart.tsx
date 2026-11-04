import {
  AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip,
  ResponsiveContainer, Legend, BarChart, Bar,
} from "recharts";
import { format } from "date-fns";
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
  const map = new Map<number, Record<string, number>>();
  activeData.forEach(({ timestamp, value }) => {
    if (!map.has(timestamp)) map.set(timestamp, { timestamp });
    map.get(timestamp)!.active = value;
  });
  removedData.forEach(({ timestamp, value }) => {
    if (!map.has(timestamp)) map.set(timestamp, { timestamp });
    map.get(timestamp)!.removed = value;
  });

  const data = Array.from(map.values())
    .sort((a, b) => a.timestamp - b.timestamp)
    .map(d => ({ ...d, label: format(new Date(d.timestamp), "HH:mm") }));

  if (data.length === 0) return (
    <div className="h-full flex items-center justify-center text-zinc-400 text-sm">No qmgr data</div>
  );

  const totalActive  = activeData.reduce((s, p) => s + p.value, 0);
  const totalRemoved = removedData.reduce((s, p) => s + p.value, 0);
  const peakActive   = Math.max(...activeData.map(p => p.value));
  const peakRemoved  = Math.max(...removedData.map(p => p.value));
  const avgActive    = activeData.length ? (totalActive / activeData.length).toFixed(1) : "0";
  const avgRemoved   = removedData.length ? (totalRemoved / removedData.length).toFixed(1) : "0";

  // Hourly bar data — bucket by hour
  const hourMap = new Map<string, { active: number; removed: number }>();
  data.forEach(d => {
    const hr = format(new Date((d as any).timestamp), "HH:00");
    if (!hourMap.has(hr)) hourMap.set(hr, { active: 0, removed: 0 });
    const b = hourMap.get(hr)!;
    b.active  += (d as any).active  ?? 0;
    b.removed += (d as any).removed ?? 0;
  });
  const hourlyData = Array.from(hourMap.entries())
    .sort((a, b) => a[0].localeCompare(b[0]))
    .map(([label, vals]) => ({ label, ...vals }));

  const statCards = [
    { label: "Total Active",   value: totalActive.toLocaleString(),  icon: Activity,     color: "text-emerald-600", bg: "bg-emerald-50",  border: "border-emerald-100" },
    { label: "Total Removed",  value: totalRemoved.toLocaleString(), icon: TrendingDown, color: "text-blue-600",    bg: "bg-blue-50",     border: "border-blue-100"    },
    { label: "Peak Active",    value: peakActive.toString(),         icon: TrendingUp,   color: "text-amber-600",   bg: "bg-amber-50",    border: "border-amber-100"   },
    { label: "Peak Removed",   value: peakRemoved.toString(),        icon: TrendingUp,   color: "text-purple-600",  bg: "bg-purple-50",   border: "border-purple-100"  },
    { label: "Avg Active/min", value: avgActive,                     icon: Minus,        color: "text-zinc-600",    bg: "bg-zinc-50",     border: "border-zinc-200"    },
    { label: "Avg Removed/min",value: avgRemoved,                    icon: Minus,        color: "text-zinc-600",    bg: "bg-zinc-50",     border: "border-zinc-200"    },
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
        <div className="px-5 py-3.5 border-b border-zinc-100 shrink-0">
          <h3 className="text-sm font-semibold text-zinc-900">Activity Over Time</h3>
          <p className="text-xs mt-0.5 text-zinc-400">Active vs removed messages per minute</p>
        </div>
        <div className="flex-1 min-h-0 p-4">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data} margin={{ top: 4, right: 8, left: 10, bottom: 0 }}>
              <defs>
                <linearGradient id="activeGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%"  stopColor="#10b981" stopOpacity={0.15} />
                  <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="removedGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%"  stopColor="#3b82f6" stopOpacity={0.15} />
                  <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="#f4f4f5" vertical={false} />
              <XAxis dataKey="label" tick={{ fill: "#a1a1aa", fontSize: 11 }} axisLine={false} tickLine={false} interval="preserveStartEnd" />
              <YAxis tick={{ fill: "#a1a1aa", fontSize: 11 }} axisLine={false} tickLine={false} tickFormatter={(v) => v >= 1000 ? `${(v/1000).toFixed(0)}k` : `${v}`} />
              <Tooltip contentStyle={tooltipStyle} cursor={{ stroke: "#e4e4e7" }} />
              <Legend wrapperStyle={{ fontSize: 12, color: "#a1a1aa", paddingTop: 8 }} />
              <Area type="monotone" dataKey="active"  name="Active"  stroke="#10b981" fill="url(#activeGrad)"  strokeWidth={1.5} dot={false} />
              <Area type="monotone" dataKey="removed" name="Removed" stroke="#3b82f6" fill="url(#removedGrad)" strokeWidth={1.5} dot={false} />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Hourly breakdown bar chart */}
      {hourlyData.length > 1 && (
        <div className="rounded-xl border border-zinc-100 bg-white shrink-0">
          <div className="px-5 py-3.5 border-b border-zinc-100">
            <h3 className="text-sm font-semibold text-zinc-900">Hourly Breakdown</h3>
            <p className="text-xs mt-0.5 text-zinc-400">Total messages active/removed per hour</p>
          </div>
          <div className="p-4">
            <ResponsiveContainer width="100%" height={160}>
              <BarChart data={hourlyData} margin={{ top: 4, right: 8, left: 10, bottom: 0 }} barCategoryGap="30%">
                <CartesianGrid strokeDasharray="3 3" stroke="#f4f4f5" vertical={false} />
                <XAxis dataKey="label" tick={{ fill: "#a1a1aa", fontSize: 11 }} axisLine={false} tickLine={false} />
                <YAxis tick={{ fill: "#a1a1aa", fontSize: 11 }} axisLine={false} tickLine={false} tickFormatter={(v) => v >= 1000 ? `${(v/1000).toFixed(0)}k` : `${v}`} />
                <Tooltip contentStyle={tooltipStyle} cursor={{ fill: "#f9f9f9" }} />
                <Legend wrapperStyle={{ fontSize: 12, color: "#a1a1aa", paddingTop: 4 }} />
                <Bar dataKey="active"  name="Active"  fill="#10b981" radius={[3, 3, 0, 0]} />
                <Bar dataKey="removed" name="Removed" fill="#3b82f6" radius={[3, 3, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      )}

    </div>
  );
}
