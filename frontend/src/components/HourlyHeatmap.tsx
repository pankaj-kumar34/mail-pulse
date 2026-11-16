import { useEffect, useState } from "react";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";
import { EmptyState } from "./EmptyState";

interface HeatmapCell {
  date: string;
  dateEpoch: number;
  hour: number;
  sent: number;
  defer: number;
  bounce: number;
  total: number;
}

interface Props {
  sinceHours: number;
  refreshKey?: number;
}

const HOURS = Array.from({ length: 24 }, (_, i) => i);

function fmtHour(h: number) {
  if (h === 0) return "12a";
  if (h < 12) return `${h}a`;
  if (h === 12) return "12p";
  return `${h - 12}p`;
}

function cellColor(cell: HeatmapCell | undefined, maxTotal: number): string {
  if (!cell || cell.total === 0) return "#f4f4f5";
  const intensity = cell.total / maxTotal;
  const failRate = cell.total > 0 ? (cell.defer + cell.bounce) / cell.total : 0;

  // Color by fail rate: high fail = rose, low fail = emerald, in-between = amber
  if (failRate > 0.6) {
    // rose ramp
    const l = Math.round(97 - intensity * 30);
    return `hsl(350, 89%, ${l}%)`;
  } else if (failRate > 0.2) {
    // amber ramp
    const l = Math.round(97 - intensity * 30);
    return `hsl(43, 96%, ${l}%)`;
  } else {
    // emerald ramp
    const l = Math.round(97 - intensity * 30);
    return `hsl(152, 76%, ${l}%)`;
  }
}

function cellTextColor(cell: HeatmapCell | undefined, maxTotal: number): string {
  if (!cell || cell.total === 0) return "transparent";
  const intensity = cell.total / maxTotal;
  return intensity > 0.5 ? "#fff" : "#52525b";
}

export function HourlyHeatmap({ sinceHours, refreshKey }: Props) {
  const [cells, setCells] = useState<HeatmapCell[]>([]);
  const [loading, setLoading] = useState(true);
  const [tooltip, setTooltip] = useState<{ cell: HeatmapCell; x: number; y: number } | null>(null);

  useEffect(() => {
    setLoading(true);
    (LogService.GetHeatmap(sinceHours) as any)
      .then((r: any) => { setCells(r ?? []); setLoading(false); })
      .catch(() => { setCells([]); setLoading(false); });
  }, [sinceHours, refreshKey]);

  if (loading) return <div className="flex items-center justify-center h-32 text-zinc-400 text-sm">Loading…</div>;
  if (!cells.length) return <EmptyState icon="inbox" title="No data" subtitle="Load a log file to see the heatmap" compact />;

  // Build grid: date rows × hour cols
  const dates = Array.from(new Set(cells.map(c => c.date)));
  const byKey = new Map(cells.map(c => [`${c.date}:${c.hour}`, c]));
  const maxTotal = Math.max(...cells.map(c => c.total), 1);

  return (
    <div style={{ position: "relative" }} onMouseLeave={() => setTooltip(null)}>
      {/* Hour labels */}
      <div style={{ display: "grid", gridTemplateColumns: `56px repeat(24, 1fr)`, gap: 2, marginBottom: 4 }}>
        <div />
        {HOURS.map(h => (
          <div key={h} style={{ fontSize: 9, color: "#a1a1aa", textAlign: "center" }}>{fmtHour(h)}</div>
        ))}
      </div>

      {/* Rows */}
      {dates.map(date => (
        <div key={date} style={{ display: "grid", gridTemplateColumns: `56px repeat(24, 1fr)`, gap: 2, marginBottom: 2 }}>
          <div style={{ fontSize: 10, color: "#71717a", display: "flex", alignItems: "center", fontWeight: 500 }}>{date}</div>
          {HOURS.map(h => {
            const cell = byKey.get(`${date}:${h}`);
            return (
              <div
                key={h}
                style={{
                  height: 22,
                  borderRadius: 3,
                  background: cellColor(cell, maxTotal),
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontSize: 8,
                  color: cellTextColor(cell, maxTotal),
                  cursor: cell ? "pointer" : "default",
                  transition: "opacity 0.1s",
                  fontVariantNumeric: "tabular-nums",
                }}
                onMouseEnter={(e) => cell && setTooltip({ cell, x: e.clientX, y: e.clientY })}
                onMouseMove={(e) => cell && setTooltip({ cell, x: e.clientX, y: e.clientY })}
              >
                {cell && cell.total > 0 ? cell.total : ""}
              </div>
            );
          })}
        </div>
      ))}

      {/* Legend */}
      <div style={{ display: "flex", alignItems: "center", gap: 16, marginTop: 12, flexWrap: "wrap" }}>
        {[
          { color: "hsl(152, 76%, 75%)", label: "Mostly sent" },
          { color: "hsl(43, 96%, 75%)",  label: "Mixed" },
          { color: "hsl(350, 89%, 75%)", label: "Mostly failed" },
          { color: "#f4f4f5",            label: "No activity" },
        ].map(({ color, label }) => (
          <div key={label} style={{ display: "flex", alignItems: "center", gap: 4, fontSize: 10, color: "#71717a" }}>
            <div style={{ width: 12, height: 12, borderRadius: 2, background: color }} />
            {label}
          </div>
        ))}
        <div style={{ fontSize: 10, color: "#a1a1aa", marginLeft: "auto" }}>Darker = more volume</div>
      </div>

      {/* Tooltip */}
      {tooltip && (
        <div style={{
          position: "fixed",
          left: tooltip.x + 12,
          top: tooltip.y - 10,
          background: "#18181b",
          color: "#fff",
          borderRadius: 6,
          padding: "6px 10px",
          fontSize: 11,
          pointerEvents: "none",
          zIndex: 100,
          whiteSpace: "nowrap",
          lineHeight: 1.6,
        }}>
          <div style={{ fontWeight: 600 }}>{tooltip.cell.date} · {fmtHour(tooltip.cell.hour)}</div>
          <div style={{ color: "#10b981" }}>✓ {tooltip.cell.sent.toLocaleString()} sent</div>
          <div style={{ color: "#eab308" }}>⟳ {tooltip.cell.defer.toLocaleString()} deferred</div>
          <div style={{ color: "#f43f5e" }}>✗ {tooltip.cell.bounce.toLocaleString()} bounced</div>
        </div>
      )}
    </div>
  );
}
