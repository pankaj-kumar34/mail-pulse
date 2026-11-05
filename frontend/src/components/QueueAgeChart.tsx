import { useEffect, useState } from "react";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";

interface QueueAgeBucket {
  label: string;
  count: number;
  percent: number;
}

interface QueueAgeData {
  buckets: QueueAgeBucket[];
  total: number;
  medianSecs: number;
  p95Secs: number;
  maxSecs: number;
}

function fmtSecs(s: number) {
  if (s < 60)   return `${s.toFixed(0)}s`;
  if (s < 3600) return `${(s / 60).toFixed(0)}m`;
  return `${(s / 3600).toFixed(1)}h`;
}

export function QueueAgeChart({ sinceHours, refreshKey }: { sinceHours: number; refreshKey?: number }) {
  const [data, setData] = useState<QueueAgeData | null>(null);

  useEffect(() => {
    LogService.GetQueueAgeDistribution(sinceHours)
      .then((r: any) => setData(r))
      .catch(() => {});
  }, [sinceHours, refreshKey]);

  if (!data || !data.total) return (
    <div className="flex-1 flex items-center justify-center text-zinc-400 text-sm">No delay data</div>
  );

  const max = Math.max(...data.buckets.map(b => b.count));

  // Emerald dark→light for fast buckets, rose light→dark for slow buckets
  const bucketColor = (i: number) => {
    const ramp = [
      "#065f46", // < 10s   — emerald-800
      "#047857", // 10–30s  — emerald-700
      "#059669", // 30s–2m  — emerald-600
      "#10b981", // 2–10m   — emerald-500
      "#34d399", // 10–30m  — emerald-400
      "#fda4af", // 30m–1h  — rose-300
      "#fb7185", // 1–6h    — rose-400
      "#f43f5e", // 6–24h   — rose-500
      "#be123c", // > 24h   — rose-700
    ];
    return ramp[i] ?? "#a1a1aa";
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: "10px" }}>

      {/* Stats row */}
      <div style={{ display: "flex", gap: "24px", marginBottom: "12px" }}>
        {[
          { label: "Total deliveries", value: data.total.toLocaleString() },
          { label: "Median delay",     value: fmtSecs(data.medianSecs) },
          { label: "P95 delay",        value: fmtSecs(data.p95Secs) },
          { label: "Max delay",        value: fmtSecs(data.maxSecs) },
        ].map(s => (
          <div key={s.label}>
            <p style={{ fontSize: "10px", fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", color: "#a1a1aa", margin: "0 0 2px 0" }}>{s.label}</p>
            <p style={{ fontSize: "16px", fontWeight: 700, color: "#3f3f46", margin: 0, fontVariantNumeric: "tabular-nums" }}>{s.value}</p>
          </div>
        ))}
      </div>

      {/* Histogram */}
      <div style={{ display: "flex", flexDirection: "column", gap: "5px" }}>
        {data.buckets.map((b, i) => (
          <div key={b.label} style={{ display: "flex", alignItems: "center", gap: "8px" }}>
            <span style={{ width: "64px", fontSize: "11px", color: "#71717a", textAlign: "right", flexShrink: 0 }}>{b.label}</span>
            <div style={{ flex: 1, height: "16px", background: "#f4f4f5", borderRadius: "3px", overflow: "hidden" }}>
              <div style={{
                height: "100%",
                width: max > 0 ? `${(b.count / max) * 100}%` : "0%",
                background: bucketColor(i),
                borderRadius: "3px",
                transition: "width 0.3s ease",
              }} />
            </div>
            <span style={{ width: "48px", fontSize: "11px", color: "#a1a1aa", textAlign: "right", flexShrink: 0, fontVariantNumeric: "tabular-nums" }}>
              {b.count > 0 ? `${b.percent.toFixed(1)}%` : "—"}
            </span>
            <span style={{ width: "40px", fontSize: "11px", color: "#71717a", flexShrink: 0, fontVariantNumeric: "tabular-nums" }}>
              {b.count > 0 ? b.count.toLocaleString() : ""}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
