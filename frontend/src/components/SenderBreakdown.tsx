import { useEffect, useState } from "react";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";

interface SenderStat {
  address: string;
  sent: number;
  deferred: number;
  bounced: number;
  total: number;
  failRate: number;
}

export function SenderBreakdown({ sinceHours, maxHeight }: { sinceHours: number; maxHeight?: number }) {
  const [rows, setRows] = useState<SenderStat[]>([]);

  useEffect(() => {
    LogService.GetSenderStats(sinceHours)
      .then((r: any) => setRows(r ?? []))
      .catch(() => {});
  }, [sinceHours]);

  if (!rows.length) return null;

  const maxTotal = Math.max(...rows.map(r => r.total), 1);

  return (
    <div className="rounded-xl border border-zinc-100 bg-white flex flex-col">
      <div className="px-5 py-3.5 border-b border-zinc-100 shrink-0">
        <h3 className="text-sm font-semibold text-zinc-800">Sender Breakdown</h3>
        <p className="text-xs mt-0.5 text-zinc-400">Delivery stats per sender address</p>
      </div>
      <div className="overflow-auto" style={{ maxHeight: maxHeight ? `${maxHeight}px` : undefined }}>
        <table style={{ width: "100%", fontSize: "11px", borderCollapse: "collapse" }}>
          <thead style={{ position: "sticky", top: 0 }}>
            <tr style={{ background: "#fafafa", borderBottom: "1px solid #f4f4f5" }}>
              {["Sender", "Sent", "Deferred", "Bounced", "Total", "Fail %"].map((h, i) => (
                <th key={h} style={{
                  padding: "6px 16px", textAlign: i === 0 ? "left" : "right",
                  fontWeight: 600, color: "#a1a1aa", fontSize: "10px",
                  letterSpacing: "0.06em", textTransform: "uppercase", whiteSpace: "nowrap",
                }}>{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.slice(0, 20).map((r, i) => {
              const failColor = r.failRate >= 80 ? "#e11d48" : r.failRate >= 50 ? "#d97706" : "#059669";
              const barPct = (r.total / maxTotal) * 100;
              return (
                <tr key={r.address} style={{ borderBottom: "1px solid #f4f4f5", background: i % 2 === 0 ? "#fff" : "#fafafa" }}>
                  <td style={{ padding: "7px 16px", maxWidth: "260px" }}>
                    <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: "#3f3f46", fontFamily: "monospace", fontSize: "11px" }}
                      title={r.address}>{r.address}</span>
                    <div style={{ height: "3px", background: "#f4f4f5", borderRadius: "2px", marginTop: "3px" }}>
                      <div style={{ height: "100%", width: `${barPct}%`, background: "#10b981", borderRadius: "2px" }} />
                    </div>
                  </td>
                  <td style={{ padding: "7px 16px", textAlign: "right", color: "#10b981", fontVariantNumeric: "tabular-nums" }}>
                    {r.sent > 0 ? r.sent.toLocaleString() : <span style={{ color: "#e4e4e7" }}>—</span>}
                  </td>
                  <td style={{ padding: "7px 16px", textAlign: "right", color: "#d97706", fontVariantNumeric: "tabular-nums" }}>
                    {r.deferred > 0 ? r.deferred.toLocaleString() : <span style={{ color: "#e4e4e7" }}>—</span>}
                  </td>
                  <td style={{ padding: "7px 16px", textAlign: "right", color: "#e11d48", fontVariantNumeric: "tabular-nums" }}>
                    {r.bounced > 0 ? r.bounced.toLocaleString() : <span style={{ color: "#e4e4e7" }}>—</span>}
                  </td>
                  <td style={{ padding: "7px 16px", textAlign: "right", color: "#71717a", fontVariantNumeric: "tabular-nums" }}>
                    {r.total.toLocaleString()}
                  </td>
                  <td style={{ padding: "7px 16px", textAlign: "right", fontWeight: 600, color: failColor, fontVariantNumeric: "tabular-nums" }}>
                    {r.failRate.toFixed(1)}%
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}
