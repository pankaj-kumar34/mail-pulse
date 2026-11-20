import { LucideIcon } from "lucide-react";

const COLOR = {
  green:  { dot: "#10b981", iconBg: "#ecfdf5", iconColor: "#059669" },
  yellow: { dot: "#f59e0b", iconBg: "#fffbeb", iconColor: "#d97706" },
  red:    { dot: "#ef4444", iconBg: "#fff1f2", iconColor: "#e11d48" },
  blue:   { dot: "#3b82f6", iconBg: "#eff6ff", iconColor: "#2563eb" },
  zinc:   { dot: "#a1a1aa", iconBg: "#f4f4f5", iconColor: "#71717a" },
};

export function MetricCard({ title, value, icon: Icon, color, suffix, subtitle, onClick }: {
  title: string;
  value: number;
  icon: LucideIcon;
  color: keyof typeof COLOR;
  suffix?: string;
  subtitle?: string;
  onClick?: () => void;
}) {
  const c = COLOR[color];
  const display = suffix
    ? `${value.toLocaleString(undefined, { maximumFractionDigits: 1 })}${suffix}`
    : value.toLocaleString();

  return (
    <div
      onClick={onClick}
      className={`mcard ${onClick ? "mcard-click" : ""}`}
      style={{ "--mcard-accent": c.dot, padding: "14px 16px", display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: "8px" } as React.CSSProperties}
    >
      <div>
        <div style={{ display: "flex", alignItems: "center", gap: "6px", marginBottom: "8px" }}>
          <span style={{ width: "6px", height: "6px", borderRadius: "50%", background: c.dot, flexShrink: 0, display: "inline-block", boxShadow: `0 0 6px ${c.dot}66` }} />
          <p style={{ fontSize: "10px", fontWeight: 600, letterSpacing: "0.08em", textTransform: "uppercase", color: "#99a0ac", margin: 0 }}>
            {title}
          </p>
        </div>
        <p style={{ fontSize: "24px", fontWeight: 700, letterSpacing: "-0.02em", color: "#161a21", lineHeight: 1, fontVariantNumeric: "tabular-nums", margin: 0 }}>
          {display}
        </p>
        {subtitle && (
          <p style={{ fontSize: "11px", color: "#99a0ac", margin: "5px 0 0 0" }}>{subtitle}</p>
        )}
      </div>
      <div style={{ padding: "7px", borderRadius: "9px", background: c.iconBg, flexShrink: 0, marginTop: "1px", boxShadow: "inset 0 0 0 1px rgba(18,22,33,.05)" }}>
        <Icon style={{ width: "15px", height: "15px", color: c.iconColor, display: "block" }} strokeWidth={2} />
      </div>
    </div>
  );
}
