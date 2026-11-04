import { useEffect, useState } from "react";
import { ArrowLeft, X } from "lucide-react";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";
import { EmptyState } from "./EmptyState";

interface ReasonMessage {
  timestamp: number;
  queueId: string;
  from: string;
  to: string;
  status: string;
  dsn: string;
  reason: string;
  message: string;
}

interface Props {
  reason: string;
  status: "deferred" | "bounced" | "";
  sinceHours: number;
  onClose: () => void;
  /** Override the header label (default: reason) */
  title?: string;
  /** Override the fetch function (default: GetMessagesByReason) */
  fetchFn?: (sinceHours: number) => Promise<ReasonMessage[]>;
}

const STATUS_COLOR: Record<string, string> = { deferred: "#d97706", bounced: "#e11d48", "": "#6366f1" };
const STATUS_BG:    Record<string, string> = { deferred: "#fffbeb", bounced: "#fff1f2", "": "#eef2ff" };

function fmt(ms: number) {
  return new Date(ms).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

export function ReasonDrilldownPanel({ reason, status, sinceHours, onClose, title, fetchFn }: Props) {
  const [rows, setRows] = useState<ReasonMessage[] | null>(null);

  useEffect(() => {
    const doFetch = fetchFn
      ? fetchFn(sinceHours)
      : LogService.GetMessagesByReason(reason, status as string, sinceHours) as Promise<ReasonMessage[]>;
    doFetch
      .then((r: any) => setRows(r ?? []))
      .catch(() => setRows([]));
  }, [reason, status, sinceHours, fetchFn]);

  const label = title ?? reason;
  const badgeLabel = status || "all";

  return (
    <div style={{
      position: "absolute", inset: 0, background: "#fff",
      borderRadius: "inherit", display: "flex", flexDirection: "column", zIndex: 10,
    }}>
      {/* Header */}
      <div style={{ display: "flex", alignItems: "center", gap: "10px", padding: "12px 16px", borderBottom: "1px solid #f4f4f5", flexShrink: 0 }}>
        <button onClick={onClose} style={{ display: "flex", alignItems: "center", gap: "4px", fontSize: "12px", color: "#71717a", background: "none", border: "none", cursor: "pointer" }}>
          <ArrowLeft size={13} /> Back
        </button>
        <span style={{
          fontSize: "10px", padding: "1px 8px", borderRadius: "4px",
          background: STATUS_BG[status] ?? "#eef2ff",
          color: STATUS_COLOR[status] ?? "#6366f1",
          fontWeight: 600,
        }}>{badgeLabel}</span>
        <span style={{ fontSize: "12px", color: "#18181b", flex: 1, fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{label}</span>
        {rows && <span style={{ fontSize: "11px", color: "#a1a1aa", flexShrink: 0 }}>{rows.length.toLocaleString()} messages</span>}
        <button onClick={onClose} style={{ background: "none", border: "none", cursor: "pointer", color: "#a1a1aa", display: "flex", flexShrink: 0 }}><X size={14} /></button>
      </div>

      {/* Table */}
      <div style={{ flex: 1, overflowY: "auto" }}>
        {!rows ? (
          <div style={{ padding: "24px", textAlign: "center", color: "#a1a1aa", fontSize: "13px" }}>Loading…</div>
        ) : rows.length === 0 ? (
          <div style={{ padding: "8px" }}><EmptyState icon="search" title="No messages found" subtitle="No matches in the selected time range" compact /></div>
        ) : (
          <table style={{ width: "100%", fontSize: "11px", borderCollapse: "collapse" }}>
            <thead style={{ position: "sticky", top: 0 }}>
              <tr style={{ background: "#fafafa", borderBottom: "1px solid #f4f4f5" }}>
                {["Time", "From", "To", "Status", "DSN", "Server Response"].map(h => (
                  <th key={h} style={{ padding: "6px 12px", textAlign: "left", fontWeight: 600, color: "#a1a1aa", fontSize: "10px", letterSpacing: "0.06em", textTransform: "uppercase", whiteSpace: "nowrap" }}>{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((r, i) => (
                <tr key={i} style={{ borderBottom: "1px solid #f4f4f5", background: i % 2 === 0 ? "#fff" : "#fafafa" }}>
                  <td style={{ padding: "6px 12px", color: "#a1a1aa", whiteSpace: "nowrap", fontVariantNumeric: "tabular-nums" }}>{fmt(r.timestamp)}</td>
                  <td style={{ padding: "6px 12px", maxWidth: "160px" }}>
                    <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: "#3f3f46" }} title={r.from}>{r.from || "—"}</span>
                  </td>
                  <td style={{ padding: "6px 12px", maxWidth: "160px" }}>
                    <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: "#3f3f46" }} title={r.to}>{r.to || "—"}</span>
                  </td>
                  <td style={{ padding: "6px 12px", whiteSpace: "nowrap" }}>
                    <span style={{
                      padding: "1px 6px", borderRadius: "4px", fontSize: "10px", fontWeight: 600,
                      background: r.status === "sent" ? "#f0fdf4" : r.status === "deferred" ? "#fffbeb" : "#fff1f2",
                      color:      r.status === "sent" ? "#16a34a" : r.status === "deferred" ? "#d97706" : "#e11d48",
                    }}>{r.status}</span>
                  </td>
                  <td style={{ padding: "6px 12px", whiteSpace: "nowrap" }}>
                    <span style={{ padding: "1px 6px", borderRadius: "4px", background: "#f4f4f5", fontFamily: "monospace", fontSize: "10px", color: "#71717a" }}>{r.dsn || "—"}</span>
                  </td>
                  <td style={{ padding: "6px 12px", color: "#52525b", maxWidth: "260px" }}>
                    <span style={{ display: "block", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={r.message}>{r.message || "—"}</span>
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
