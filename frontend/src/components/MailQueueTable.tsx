import { useState, useEffect, useCallback } from "react";
import { Search, RefreshCw, Loader2, ArrowUpDown, ArrowUp, ArrowDown, ChevronLeft, ChevronRight, Mail } from "lucide-react";
import { MailQueueService } from "../../bindings/github.com/pankajkumar/mail-pulse";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/ui/badge";
import { NativeSelect } from "@/components/ui/select";

interface Recipient { address: string }
interface QueueEntry { queue_name: string; queue_id: string; arrival_time: number; message_size: number; sender: string; recipients: Recipient[] }
interface Pagination { page: number; limit: number; total: number; totalPages: number; hasNext: boolean; hasPrev: boolean }

const QUEUE_BADGE: Record<string, any> = { deferred: "yellow", active: "green", hold: "blue", corrupt: "red", incoming: "blue" };
const fmtBytes = (b: number) => b < 1024 ? `${b} B` : b < 1048576 ? `${(b/1024).toFixed(1)} KB` : `${(b/1048576).toFixed(1)} MB`;
const fmtArrival = (ts: number) => new Date(ts * 1000).toLocaleString("en-GB", {
  day: "2-digit", month: "short", hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false,
});

type SortField = "queue_id" | "queue_name" | "message_size" | "arrival_time" | "sender";

function SortIcon({ field, sortBy, order }: { field: string; sortBy: string; order: string }) {
  if (sortBy !== field) return <ArrowUpDown className="w-3 h-3 opacity-30 inline ml-1" />;
  return order === "asc"
    ? <ArrowUp className="w-3 h-3 inline ml-1" style={{ color: "var(--accent)" }} />
    : <ArrowDown className="w-3 h-3 inline ml-1" style={{ color: "var(--accent)" }} />;
}

export function MailQueueTable({ refreshKey }: { refreshKey: number }) {
  const [entries, setEntries]       = useState<QueueEntry[]>([]);
  const [pagination, setPagination] = useState<Pagination>({ page: 1, limit: 20, total: 0, totalPages: 0, hasNext: false, hasPrev: false });
  const [search, setSearch]         = useState("");
  const [queue, setQueue]           = useState("");
  const [sortBy, setSortBy]         = useState<SortField>("arrival_time");
  const [sortOrder, setSortOrder]   = useState<"asc" | "desc">("desc");
  const [loading, setLoading]       = useState(false);
  const [qError, setQError]         = useState<string | null>(null);

  const fetchQueue = useCallback(async (page: number) => {
    setLoading(true); setQError(null);
    try {
      const r = await MailQueueService.GetQueue(page, 20, search, queue, sortBy, sortOrder) as any;
      setEntries(r.entries ?? []);
      setPagination(r.pagination ?? { page: 1, limit: 20, total: 0, totalPages: 0, hasNext: false, hasPrev: false });
    } catch (e) { setQError("Queue unavailable"); console.error(e); }
    finally { setLoading(false); }
  }, [search, queue, sortBy, sortOrder]);

  useEffect(() => { if (refreshKey === 0) return; fetchQueue(1); }, [fetchQueue, refreshKey]);

  const handleSort = (f: SortField) => {
    if (sortBy === f) setSortOrder(o => o === "asc" ? "desc" : "asc");
    else { setSortBy(f); setSortOrder("desc"); }
  };

  const start = pagination.total === 0 ? 0 : (pagination.page - 1) * pagination.limit + 1;
  const end   = Math.min(pagination.page * pagination.limit, pagination.total);

  const Th = ({ field, label }: { field: SortField; label: string }) => (
    <th
      className="px-5 py-2.5 text-left text-[11px] font-semibold uppercase tracking-wider cursor-pointer select-none"
      style={{ color: "#a1a1aa" }}
      onClick={() => handleSort(field)}
    >
      {label}<SortIcon field={field} sortBy={sortBy} order={sortOrder} />
    </th>
  );

  return (
    <div className="flex flex-col h-full bg-white">
      {/* Toolbar */}
      <div className="px-5 py-3 border-b flex items-center gap-3 shrink-0"
        style={{ borderColor: "#e4e4e7" }}>
        {qError && <span className="text-xs" style={{ color: "var(--c-yellow)" }}>{qError}</span>}
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 pointer-events-none"
            style={{ color: "#a1a1aa" }} />
          <Input placeholder="Search queue ID, sender…" value={search} onChange={e => setSearch(e.target.value)} className="pl-10" />
        </div>
        <NativeSelect value={queue || ""} onChange={e => setQueue(e.target.value)}>
          <option value="">All Queues</option>
          <option value="deferred">Deferred</option>
          <option value="active">Active</option>
          <option value="hold">Hold</option>
          <option value="incoming">Incoming</option>
        </NativeSelect>
        <button
          onClick={() => fetchQueue(pagination.page)}
          className="h-9 w-9 flex items-center justify-center rounded-lg border transition-colors cursor-pointer hover:bg-zinc-50"
          style={{ borderColor: "#e4e4e7" }}
          title="Refresh"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${loading ? "animate-spin" : ""}`}
            style={{ color: loading ? "var(--accent)" : "var(--content-text-sub)" }} />
        </button>
      </div>

      {/* Table */}
      <div className="flex-1 overflow-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b" style={{ borderColor: "#e4e4e7" }}>
              <Th field="queue_id"     label="Queue ID" />
              <Th field="queue_name"   label="Queue" />
              <Th field="message_size" label="Size" />
              <Th field="arrival_time" label="Arrival Time" />
              <Th field="sender"       label="Sender" />
              <th className="px-5 py-2.5 text-left text-[11px] font-semibold uppercase tracking-wider"
                style={{ color: "#a1a1aa" }}>Recipients</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr><td colSpan={6} className="py-20 text-center">
                <Loader2 className="w-5 h-5 animate-spin mx-auto" style={{ color: "#e4e4e7" }} />
              </td></tr>
            ) : entries.length === 0 ? (
              <tr><td colSpan={6} className="py-20 text-center">
                <Mail className="w-10 h-10 mx-auto mb-3 text-zinc-200" strokeWidth={1} />
                <p className="text-sm font-medium" style={{ color: "#a1a1aa" }}>Queue is empty</p>
                <p className="text-xs mt-1" style={{ color: "#a1a1aa" }}>No messages in the queue</p>
              </td></tr>
            ) : entries.map(e => (
              <tr key={e.queue_id} className="border-b hover:bg-zinc-50 transition-colors"
                style={{ borderColor: "#f4f4f5" }}>
                <td className="px-5 py-3 font-mono text-xs" style={{ color: "#3f3f46" }}>{e.queue_id}</td>
                <td className="px-5 py-3">
                  <StatusBadge variant={QUEUE_BADGE[e.queue_name] ?? "secondary"}>{e.queue_name}</StatusBadge>
                </td>
                <td className="px-5 py-3 text-xs tabular-nums" style={{ color: "#71717a" }}>{fmtBytes(e.message_size)}</td>
                <td className="px-5 py-3 text-xs whitespace-nowrap" style={{ color: "#71717a" }}>{fmtArrival(e.arrival_time)}</td>
                <td className="px-5 py-3 text-xs max-w-[200px] truncate" style={{ color: "#3f3f46" }}>{e.sender}</td>
                <td className="px-5 py-3 text-xs" style={{ color: "#71717a" }}>
                  {e.recipients?.length ? e.recipients.map(r => r.address).join(", ") : "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Footer */}
      <div className="px-5 py-3 border-t flex items-center justify-between shrink-0"
        style={{ borderColor: "#e4e4e7", background: "#fafafa" }}>
        <span className="text-xs" style={{ color: "#a1a1aa" }}>
          {pagination.total === 0 ? "Empty queue" : `${start}–${end} of ${pagination.total}`}
        </span>
        <div className="flex items-center gap-1">
          <Button variant="ghost" size="icon-sm" disabled={!pagination.hasPrev} onClick={() => fetchQueue(pagination.page - 1)}>
            <ChevronLeft className="w-4 h-4" />
          </Button>
          <Button variant="ghost" size="icon-sm" disabled={!pagination.hasNext} onClick={() => fetchQueue(pagination.page + 1)}>
            <ChevronRight className="w-4 h-4" />
          </Button>
        </div>
      </div>
    </div>
  );
}
