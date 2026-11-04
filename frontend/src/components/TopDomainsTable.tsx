interface DomainCount { domain: string; count: number }

// Log scale so small values still show a meaningful bar
function barWidth(count: number, max: number): string {
  if (max === 0) return "0%";
  // sqrt scale: compresses big values, amplifies small ones
  const pct = Math.sqrt(count / max) * 100;
  return `${Math.max(pct, 2).toFixed(1)}%`;
}

export function TopDomainsTable({ data, title, color, maxHeight }: {
  data: DomainCount[]; title: string; color: "blue" | "green"; maxHeight?: number;
}) {
  const barColor  = color === "blue" ? "#3b82f6" : "#10b981";
  const badgeBg   = color === "blue" ? "#dbeafe" : "#d1fae5";
  const badgeText = color === "blue" ? "#1e3a8a" : "#065f46";
  const max       = data[0]?.count ?? 1;
  const total     = data.reduce((s, d) => s + d.count, 0);

  return (
    <div style={{ background: "#fff", borderRadius: "12px", border: "1px solid #e4e4e7", boxShadow: "none", display: "flex", flexDirection: "column", overflow: "hidden", height: "100%" }}>

      <div style={{ padding: "16px 20px", borderBottom: "1px solid #f4f4f5", flexShrink: 0 }}>
        <p style={{ fontSize: "13px", fontWeight: 600, color: "#18181b", margin: 0 }}>{title}</p>
        <p style={{ fontSize: "11px", color: "#a1a1aa", margin: "2px 0 0 0" }}>Top 20 domains by message count</p>
      </div>

      <div style={{ overflow: "auto", flex: 1, minHeight: 0, maxHeight: maxHeight ? `${maxHeight}px` : undefined }}>
        <table style={{ width: "100%", borderCollapse: "collapse", fontSize: "13px" }}>
          <thead>
            <tr style={{ borderBottom: "1px solid #f4f4f5" }}>
              {[["#","left","32px"],["DOMAIN","left","auto"],["COUNT","right","80px"],["%","left","140px"]].map(([h,a,w]) => (
                <th key={h} style={{ padding: "8px 16px", textAlign: a as any, fontSize: "11px", fontWeight: 600, letterSpacing: "0.05em", textTransform: "uppercase", color: "#a1a1aa", width: w }}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {data.length === 0 ? (
              <tr><td colSpan={4} style={{ padding: "40px", textAlign: "center", color: "#a1a1aa", fontSize: "13px" }}>No data</td></tr>
            ) : data.map((d, i) => {
              const pct = total > 0 ? (d.count / total) * 100 : 0;
              const bw  = barWidth(d.count, max);
              return (
                <tr key={d.domain} style={{ borderBottom: "1px solid #fafafa" }}
                  onMouseEnter={e => (e.currentTarget as HTMLElement).style.background = "#fafafa"}
                  onMouseLeave={e => (e.currentTarget as HTMLElement).style.background = "transparent"}>
                  <td style={{ padding: "9px 16px", color: "#a1a1aa", fontSize: "12px", fontVariantNumeric: "tabular-nums" }}>{i + 1}</td>
                  <td style={{ padding: "9px 16px", fontWeight: 500, color: "#18181b", maxWidth: "180px", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{d.domain}</td>
                  <td style={{ padding: "9px 16px", textAlign: "right" }}>
                    <span style={{ display: "inline-flex", alignItems: "center", justifyContent: "center", padding: "1px 8px", borderRadius: "999px", fontSize: "11px", fontWeight: 600, fontVariantNumeric: "tabular-nums", background: badgeBg, color: badgeText }}>
                      {d.count.toLocaleString()}
                    </span>
                  </td>
                  <td style={{ padding: "9px 16px" }}>
                    <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                      <div style={{ flex: 1, height: "6px", borderRadius: "999px", background: "#f4f4f5", position: "relative" }}>
                        <div style={{ position: "absolute", left: 0, top: 0, bottom: 0, borderRadius: "999px", background: barColor, width: bw, transition: "width 0.3s ease" }} />
                      </div>
                      <span style={{ fontSize: "11px", color: "#a1a1aa", fontVariantNumeric: "tabular-nums", width: "36px", textAlign: "right", flexShrink: 0 }}>
                        {pct.toFixed(1)}%
                      </span>
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {data.length > 0 && (
        <div style={{ padding: "8px 20px", borderTop: "1px solid #f4f4f5", background: "#fafafa", display: "flex", justifyContent: "space-between" }}>
          <span style={{ fontSize: "11px", color: "#a1a1aa" }}>Domains: <strong style={{ color: "#52525b" }}>{data.length}</strong></span>
          <span style={{ fontSize: "11px", color: "#a1a1aa" }}>Total: <strong style={{ color: "#52525b" }}>{total.toLocaleString()}</strong></span>
        </div>
      )}
    </div>
  );
}
