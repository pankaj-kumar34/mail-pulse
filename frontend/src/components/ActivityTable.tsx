import { useState, useEffect, useCallback } from "react";
import { Search, ChevronLeft, ChevronRight, Loader2, ArrowUpDown, ArrowUp, ArrowDown } from "lucide-react";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";
import { EmptyState } from "./EmptyState";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/ui/badge";
import { NativeSelect } from "@/components/ui/select";

interface MessageRow {
  queueId: string;
  time: number;
  from: string;
  to: string;
  status: string;
  delay: number;
  processTime: number;
}
interface Pagination { page: number; limit: number; total: number; totalPages: number; hasNext: boolean; hasPrev: boolean }

function maskEmail(email?: string): string {
  if (!email) return "—";
  const at = email.indexOf("@");
  if (at < 0) return email.slice(0, 2) + "***";
  const local = email.slice(0, at);
  const domain = email.slice(at);
  const visible = Math.min(2, local.length);
  return local.slice(0, visible) + "*".repeat(Math.max(3, local.length - visible)) + domain;
}

const STATUS_VARIANT: Record<string, string> = {
  sent: "green", deferred: "yellow", bounced: "red", removed: "secondary", active: "blue",
};

function SortIcon({ field, sortBy, order }: { field: string; sortBy: string; order: string }) {
  if (sortBy !== field) return <ArrowUpDown className="w-3 h-3 opacity-25 inline ml-1" />;
  return order === "asc"
    ? <ArrowUp   className="w-3 h-3 inline ml-1" style={{ color: "#10b981" }} />
    : <ArrowDown className="w-3 h-3 inline ml-1" style={{ color: "#10b981" }} />;
}

function fmtDelay(s: number): string {
  if (!s || s <= 0) return "—";
  if (s < 60) return `${s.toFixed(2)}s`;
  const m = Math.floor(s / 60);
  const sec = (s % 60).toFixed(0);
  return `${m}m ${sec}s`;
}

export function ActivityTable({ refreshKey, sinceHours = 0, initialStatus = "", filterSeq = 0 }: { refreshKey: number; sinceHours?: number; initialStatus?: string; filterSeq?: number }) {
  const [rows, setRows]              = useState<MessageRow[]>([]);
  const [pagination, setPagination]  = useState<Pagination>({ page: 1, limit: 50, total: 0, totalPages: 0, hasNext: false, hasPrev: false });
  const [search, setSearch]          = useState("");
  const [status, setStatus]          = useState(initialStatus);
  const [sortBy, setSortBy]          = useState("time");
  const [sortOrder, setSortOrder]    = useState<"asc" | "desc">("desc");
  const [loading, setLoading]        = useState(false);

  // Sync status when parent navigates here with a new filter
  useEffect(() => { setStatus(initialStatus); }, [initialStatus, filterSeq]);

  const fetchRows = useCallback(async (page: number) => {
    setLoading(true);
    try {
      const r = await LogService.GetMessages(sinceHours, page, 50, search, status, sortBy, sortOrder) as any;
      setRows(r.rows ?? []);
      setPagination(r.pagination ?? { page: 1, limit: 50, total: 0, totalPages: 0, hasNext: false, hasPrev: false });
    } catch (e) { console.error(e); }
    finally { setLoading(false); }
  }, [sinceHours, search, status, sortBy, sortOrder]);

  useEffect(() => { if (refreshKey === 0) return; fetchRows(1); }, [fetchRows, refreshKey]);

  const handleSort = (field: string) => {
    if (sortBy === field) setSortOrder(o => o === "asc" ? "desc" : "asc");
    else { setSortBy(field); setSortOrder("desc"); }
  };

  const Th = ({ field, label, className = "" }: { field: string; label: string; className?: string }) => (
    <th
      className={`px-4 py-3 text-left text-[11px] font-semibold uppercase tracking-wider cursor-pointer select-none whitespace-nowrap ${className}`}
      style={{ color: "#a1a1aa" }}
      onClick={() => handleSort(field)}
    >
      {label}<SortIcon field={field} sortBy={sortBy} order={sortOrder} />
    </th>
  );

  return (
    <div className="flex flex-col h-full bg-white">

      {/* Toolbar */}
      <div className="px-5 py-3 shrink-0 flex items-center gap-3"
        style={{ borderBottom: "1px solid #e4e4e7" }}>
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 pointer-events-none"
            style={{ color: "#a1a1aa" }} />
          <Input placeholder="Search queue ID, sender, recipient…" value={search}
            onChange={e => setSearch(e.target.value)} className="pl-10" />
        </div>
        <NativeSelect value={status} onChange={e => setStatus(e.target.value)}>
          <option value="">All statuses</option>
          <option value="sent">Sent</option>
          <option value="deferred">Deferred</option>
          <option value="bounced">Bounced</option>
        </NativeSelect>
      </div>

      {/* Table */}
      <div className="overflow-auto flex-1">
        <table className="w-full text-sm table-fixed">
          <colgroup>
            <col style={{ width: "80px" }} />   {/* Time */}
            <col style={{ width: "110px" }} />  {/* Queue ID */}
            <col style={{ width: "90px" }} />   {/* Status */}
            <col style={{ width: "220px" }} />  {/* From */}
            <col style={{ width: "220px" }} />  {/* To */}
            <col style={{ width: "100px" }} />  {/* Process Time */}
          </colgroup>
          <thead>
            <tr style={{ borderBottom: "1px solid #f4f4f5", background: "#fafafa" }}>
              <Th field="time"        label="Time" />
              <Th field="queueId"     label="Queue ID" />
              <Th field="status"      label="Status" />
              <Th field="from"        label="From" />
              <Th field="to"          label="To" />
              <Th field="processTime" label="Process Time" />
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr><td colSpan={6} className="py-16 text-center">
                <Loader2 className="w-5 h-5 animate-spin mx-auto" style={{ color: "#d4d4d8" }} />
              </td></tr>
            ) : rows.length === 0 ? (
              <tr><td colSpan={6} className="py-2">
                <EmptyState icon="mail" title="No messages found" subtitle="Try adjusting your filters" compact />
              </td></tr>
            ) : rows.map((row, i) => (
              <tr key={`${row.queueId}-${row.to}-${i}`}
                className="transition-colors hover:bg-zinc-50/60"
                style={{ borderBottom: "1px solid #f4f4f5" }}>

                {/* Time */}
                <td className="px-4 py-2.5 text-xs tabular-nums whitespace-nowrap" style={{ color: "#71717a" }}>
                  {new Date(row.time).toLocaleTimeString("en-GB", { hour12: false })}
                </td>

                {/* Queue ID */}
                <td className="px-4 py-2.5 font-mono text-xs" style={{ color: "#3f3f46" }}>
                  {row.queueId || "—"}
                </td>

                {/* Status */}
                <td className="px-4 py-2.5">
                  {row.status
                    ? <StatusBadge variant={STATUS_VARIANT[row.status] ?? "secondary"}>{row.status}</StatusBadge>
                    : <span style={{ color: "#d4d4d8" }}>—</span>}
                </td>

                {/* From */}
                <td className="px-4 py-2.5 text-xs truncate" style={{ color: "#52525b" }}>
                  {maskEmail(row.from)}
                </td>

                {/* To */}
                <td className="px-4 py-2.5 text-xs truncate" style={{ color: "#3f3f46" }}>
                  {maskEmail(row.to)}
                </td>

                {/* Process time */}
                <td className="px-4 py-2.5 text-xs tabular-nums" style={{ color: "#71717a" }}>
                  {fmtDelay(row.processTime)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Footer */}
      <div className="px-5 py-3 shrink-0 flex items-center justify-between"
        style={{ borderTop: "1px solid #e4e4e7", background: "#fafafa" }}>
        <span className="text-xs" style={{ color: "#a1a1aa" }}>
          {pagination.total.toLocaleString()} messages
          {pagination.totalPages > 1 && ` · Page ${pagination.page} of ${pagination.totalPages}`}
        </span>
        <div className="flex items-center gap-1">
          <Button variant="ghost" size="icon-sm" disabled={!pagination.hasPrev}
            onClick={() => fetchRows(pagination.page - 1)}>
            <ChevronLeft className="w-4 h-4" />
          </Button>
          <Button variant="ghost" size="icon-sm" disabled={!pagination.hasNext}
            onClick={() => fetchRows(pagination.page + 1)}>
            <ChevronRight className="w-4 h-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
