import { useEffect, useState } from "react";
import { X, ArrowLeft } from "lucide-react";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";

interface RecipientStat {
  address: string;
  domain: string;
  sent: number;
  deferred: number;
  bounced: number;
  total: number;
  failRate: number;
  lastSeen: number;
  lastStatus: string;
  lastReason: string;
}

interface Props {
  domain: string;
  sinceHours: number;
  onClose: () => void;
}

const STATUS_COLOR: Record<string, string> = {
  sent:     "#10b981",
  deferred: "#f59e0b",
  bounced:  "#ef4444",
};

const STATUS_BG: Record<string, string> = {
  sent:     "#f0fdf4",
  deferred: "#fffbeb",
  bounced:  "#fff1f2",
};

function fmt(ms: number) {
  if (!ms) return "—";
  return new Date(ms).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

export function RecipientDrilldownPanel({ domain, sinceHours, onClose }: Props) {
  const [data, setData] = useState<{ recipients: RecipientStat[]; sent: number; deferred: number; bounced: number; total: number; failRate: number } | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    LogService.GetRecipientDrilldown(domain, sinceHours)
      .then((r: any) => { setData(r); setLoading(false); })
      .catch(() => setLoading(false));
  }, [domain, sinceHours]);

  return (
    <div style={{
      position: "absolute", inset: 0, background: "#fff",
      borderRadius: "inherit", display: "flex", flexDirection: "column",
      zIndex: 10,
    }}>
      {/* Header */}
      <div style={{
        display: "flex", alignItems: "center", gap: "10px",
        padding: "12px 16px", borderBottom: "1px solid #f4f4f5", flexShrink: 0,
      }}>
        <button onClick={onClose} style={{
          display: "flex", alignItems: "center", gap: "4px",
          fontSize: "12px", color: "#71717a", background: "none",
          border: "none", cursor: "pointer", padding: "2px 4px",
        }}>
          <ArrowLeft size={13} /> Back
        </button>
        <span style={{ fontSize: "13px", fontWeight: 600, color: "#3f3f46", flex: 1 }}>{domain}</span>
        {data && (
          <span style={{ fontSize: "11px", color: "#a1a1aa" }}>
            {data.total.toLocaleString()} deliveries · {data.failRate.toFixed(1)}% fail rate
          </span>
        )}
        <button onClick={onClose} style={{ background: "none", border: "none", cursor: "pointer", color: "#a1a1aa", display: "flex" }}>
          <X size={14} />
        </button>
      </div>

      {/* Body */}
      <div style={{ flex: 1, overflowY: "auto" }}>
        {loading ? (
          <div style={{ padding: "24px", textAlign: "center", color: "#a1a1aa", fontSize: "13px" }}>Loading…</div>
        ) : !data?.recipients?.length ? (
          <div style={{ padding: "24px", textAlign: "center", color: "#a1a1aa", fontSize: "13px" }}>No recipients found</div>
        ) : (
          <table style={{ width: "100%", fontSize: "11px", borderCollapse: "collapse" }}>
            <thead style={{ position: "sticky", top: 0, zIndex: 1 }}>
              <tr style={{ background: "#fafafa", borderBottom: "1px solid #f4f4f5" }}>
                {["Recipient", "Sent", "Deferred", "Bounced", "Last", "Reason"].map(h => (
                  <th key={h} style={{
                    padding: "6px 10px", textAlign: h === "Sent" || h === "Deferred" || h === "Bounced" ? "right" : "left",
                    fontWeight: 600, color: "#a1a1aa", fontSize: "10px",
                    letterSpacing: "0.06em", textTransform: "uppercase", whiteSpace: "nowrap",
                  }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {data.recipients.map((r, i) => (
                <tr key={r.address} style={{
                  borderBottom: "1px solid #f4f4f5",
                  background: i % 2 === 0 ? "#fff" : "#fafafa",
                }}>
                  <td style={{ padding: "6px 10px", color: "#3f3f46", maxWidth: "180px" }}>
                    <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
                      title={r.address}>{r.address}</span>
                  </td>
                  <td style={{ padding: "6px 10px", textAlign: "right", color: "#10b981", fontVariantNumeric: "tabular-nums" }}>
                    {r.sent > 0 ? r.sent.toLocaleString() : <span style={{ color: "#e4e4e7" }}>—</span>}
                  </td>
                  <td style={{ padding: "6px 10px", textAlign: "right", color: "#f59e0b", fontVariantNumeric: "tabular-nums" }}>
                    {r.deferred > 0 ? r.deferred.toLocaleString() : <span style={{ color: "#e4e4e7" }}>—</span>}
                  </td>
                  <td style={{ padding: "6px 10px", textAlign: "right", color: "#ef4444", fontVariantNumeric: "tabular-nums" }}>
                    {r.bounced > 0 ? r.bounced.toLocaleString() : <span style={{ color: "#e4e4e7" }}>—</span>}
                  </td>
                  <td style={{ padding: "6px 10px", whiteSpace: "nowrap" }}>
                    <span style={{
                      fontSize: "10px", padding: "1px 6px", borderRadius: "4px",
                      background: STATUS_BG[r.lastStatus] ?? "#f4f4f5",
                      color: STATUS_COLOR[r.lastStatus] ?? "#71717a",
                      fontWeight: 500,
                    }}>{r.lastStatus}</span>
                    <span style={{ color: "#a1a1aa", marginLeft: "4px" }}>{fmt(r.lastSeen)}</span>
                  </td>
                  <td style={{ padding: "6px 10px", color: "#71717a", maxWidth: "140px" }}>
                    <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
                      title={r.lastReason}>{r.lastReason || "—"}</span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
