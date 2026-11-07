import { useState, useEffect } from "react";
import {
  Mail, Clock, AlertCircle, AlertTriangle, RefreshCw,
  FolderOpen, LayoutDashboard, Activity, BarChart2, TrendingUp, Inbox, ScrollText,
  CheckCircle2, BarChart, Network, Users, Cloud,
} from "lucide-react";
import { usePostfixLogs, GetMetrics, GetDelayInsights } from "../hooks/usePostfixLogs";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";
import { MetricCard } from "./MetricCard";
import DelayInsightsChart from "./DelayInsightsChart";
import { RetryTimelinePanel } from "./RetryTimelinePanel";
import { QueueSummaryTable } from "./QueueSummaryTable";
import { QmgrActivityChart } from "./QmgrActivityChart";
import { ActivityTable } from "./ActivityTable";
import { ErrorLogViewer } from "./ErrorLogViewer";
import { IPReputationPanel } from "./IPReputationPanel";
import { MailQueueTable } from "./MailQueueTable";
import { TopDomainsTable } from "./TopDomainsTable";
import { RawLogViewer } from "./RawLogViewer";
import { VolumeInsightsChart } from "./VolumeInsightsChart";
import { RelayInsightsPanel } from "./RelayInsightsPanel";
import LogSourcePanel from "./LogSourcePanel";
import { NativeSelect } from "@/components/ui/select";

type Tab = "overview" | "volume" | "insights" | "activity" | "queuesummary" | "qmgr" | "queue" | "relays" | "errors" | "rawlog" | "logsource";

interface Metrics {
  totalSent: number; totalDeferred: number; totalBounced: number;
  totalRemoved: number; errorCount: number; warningCount: number;
  averageDelay: number; deliverySuccessRate: number;
  topSenderDomains: { domain: string; count: number }[];
  topRecipientDomains: { domain: string; count: number }[];
  qmgrActiveHistory: { timestamp: number; value: number }[];
  qmgrRemovedHistory: { timestamp: number; value: number }[];
}

const NAV: { id: Tab; label: string; icon: React.ElementType }[] = [
  { id: "overview",  label: "Overview",      icon: LayoutDashboard },
  { id: "volume",    label: "Volume",         icon: BarChart        },
  { id: "insights",  label: "Insights",       icon: TrendingUp      },
  { id: "activity",     label: "Activity",      icon: Activity        },
  { id: "queuesummary", label: "Queue Summary", icon: Users           },
  { id: "qmgr",         label: "Qmgr",          icon: BarChart2       },
  { id: "queue",     label: "Mail Queue",     icon: Inbox           },
  { id: "relays",    label: "Relays",         icon: Network         },
  { id: "errors",    label: "Diagnostics",    icon: AlertCircle     },
  { id: "rawlog",    label: "Raw Log",        icon: ScrollText      },
  { id: "logsource", label: "Log Source",     icon: Cloud           },
];

const TAB_TITLE: Record<Tab, string> = {
  overview:     "Overview",
  volume:       "Volume & Delivery",
  insights:     "Delay Insights",
  activity:     "Activity Log",
  queuesummary: "Queue Summary",
  qmgr:         "Queue Manager",
  queue:         "Mail Queue",
  relays:        "Relay Insights",
  errors:        "Diagnostics",
  rawlog:        "Raw Log",
  logsource:     "Log Source",
};

export default function Dashboard() {
  const {
    isConnected, isLoading, error, reconnect,
    refreshFile, refreshKey, selectedFile,
    availableFiles, fetchAvailableFiles,
  } = usePostfixLogs();

  const [tab, setTab]                         = useState<Tab>("overview");
  const [mountedTabs, setMountedTabs]         = useState<Set<Tab>>(new Set(["overview"]));
  const [currentTime, setCurrentTime]         = useState("");
  const [metrics, setMetrics]                 = useState<Metrics | null>(null);
  const [delayInsights, setDelayInsights]     = useState<any>(null);
  const [volumeInsights, setVolumeInsights]   = useState<any>(null);
  const [sinceHours, setSinceHours]           = useState<number>(0); // 0 = all time
  const [logTimeRange, setLogTimeRange]       = useState<{ min: number; max: number } | null>(null);
  const [activityFilter, setActivityFilter]   = useState<string>(""); // pre-filter for activity tab
  const [activityFilterSeq, setActivityFilterSeq] = useState(0);    // bumped on every nav to force effect
  const [diagFilter, setDiagFilter]           = useState<"all"|"errors"|"warnings">("all");

  // Mount tab on first visit — never unmount (lazy mount pattern)
  const switchTab = (t: Tab) => {
    setTab(t);
    setMountedTabs(prev => new Set([...prev, t]));
  };

  const goToActivity = (status: string) => {
    setActivityFilter(status);
    setActivityFilterSeq(s => s + 1);
    switchTab("activity");
  };

  const goToDiag = (type: "all"|"errors"|"warnings") => {
    setDiagFilter(type);
    switchTab("errors");
  };

  useEffect(() => {
    const tick = () => setCurrentTime(new Date().toLocaleTimeString("en-US", { hour12: false }));
    tick(); const id = setInterval(tick, 1000); return () => clearInterval(id);
  }, []);

  useEffect(() => { void fetchAvailableFiles(); }, [fetchAvailableFiles]);

  const loadData = async (hours: number) => {
    try {
      const [m, d, v, tr] = await Promise.all([
        GetMetrics(hours),
        GetDelayInsights(hours),
        LogService.GetVolumeInsights(hours),
        LogService.GetLogTimeRange(),
      ]);
      setMetrics(m as unknown as Metrics);
      setDelayInsights(d);
      setVolumeInsights(v);
      setLogTimeRange(tr as unknown as { min: number; max: number });
    } catch (e) { console.error(e); }
  };

  useEffect(() => {
    if (refreshKey === 0) return;
    void loadData(sinceHours);
  }, [refreshKey]);

  // Re-fetch when range changes (without needing a full re-ingest)
  useEffect(() => {
    if (refreshKey === 0) return;
    void loadData(sinceHours);
  }, [sinceHours]);

  

  return (
    <div className="flex overflow-hidden p-2.5 gap-2" style={{ background: "#d8d8de", height: "100%" }}>

      {/* ── Sidebar panel ───────────────────────────────────────────────── */}
      <aside className="w-[210px] shrink-0 flex flex-col rounded-2xl overflow-hidden"
        style={{ background: "#1e1e20", boxShadow: "inset 0 0 0 1px rgba(255,255,255,.07)" }}>

        {/* macOS traffic light drag zone — sits above brand */}
        <div className="h-5 shrink-0"
          style={{ WebkitAppRegion: "drag" } as React.CSSProperties} />

        {/* Brand */}
        <div className="px-4 pb-3 mb-1">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2.5">
              <div className="w-7 h-7 rounded-lg flex items-center justify-center shrink-0"
                style={{ background: "rgba(16,185,129,0.18)" }}>
                <Mail className="w-3.5 h-3.5" style={{ color: "#10b981" }} />
              </div>
              <div>
                <p className="text-[13px] font-bold text-white leading-tight">Mail Pulse</p>
                <p className="text-[9px] font-semibold tracking-[0.12em] text-zinc-500">POSTFIX MONITOR</p>
              </div>
            </div>
          </div>
        </div>

        {/* Divider */}
        <div className="mx-3 mb-2" style={{ height: "1px", background: "rgba(255,255,255,.08)" }} />

        {/* Range selector — right under brand, always visible */}
        <div style={{ padding: "0 12px 10px", flexShrink: 0 }}>
          <div style={{ display: "flex", border: "1px solid rgba(255,255,255,.2)", borderRadius: "8px", overflow: "hidden", background: "rgba(0,0,0,.35)" }}>
            {["1h","6h","24h","All"].map((label, i) => {
              const hrs = [1,6,24,0][i];
              return (
                <button
                  key={label}
                  onClick={() => setSinceHours(hrs)}
                  style={{
                    flex: 1, height: "28px", fontSize: "11px", fontWeight: 600,
                    cursor: "pointer",
                    background: sinceHours === hrs ? "rgba(16,185,129,0.3)" : "transparent",
                    color:      sinceHours === hrs ? "#10b981" : "#71717a",
                    border: "none",
                    borderRight: i < 3 ? "1px solid rgba(255,255,255,.08)" : "none",
                  }}
                >
                  {label}
                </button>
              );
            })}
          </div>
          {/* Show actual window being displayed */}
          {logTimeRange && logTimeRange.max > 0 && (() => {
            const anchor = logTimeRange.max;
            const since  = sinceHours > 0 ? anchor - sinceHours * 3600000 : logTimeRange.min;
            const fmt = (ms: number) => new Date(ms).toLocaleString("en-GB", {
              month: "short", day: "numeric", hour: "2-digit", minute: "2-digit", hour12: false,
            });
            return (
              <p style={{ fontSize: "10px", color: "#a1a1aa", marginTop: "5px", textAlign: "center" }}>
                {fmt(since)} – {fmt(anchor)}
              </p>
            );
          })()}
        </div>

        {/* Nav */}
        <nav className="flex-1 overflow-y-auto px-2 space-y-0.5 min-h-0"
          style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}>
          {NAV.map(item => {
            const active = tab === item.id;
            return (
              <button
                key={item.id}
                onClick={() => switchTab(item.id)}
                className="flex items-center gap-2.5 w-full px-3 py-2 rounded-lg text-[13px] font-medium transition-all cursor-pointer text-left"
                style={{
                  background: active ? "rgba(16,185,129,0.15)" : "transparent",
                  color:      active ? "#10b981" : "#a1a1aa",
                }}
                onMouseEnter={e => { if (!active) (e.currentTarget as HTMLElement).style.background = "rgba(255,255,255,.06)"; }}
                onMouseLeave={e => { if (!active) (e.currentTarget as HTMLElement).style.background = "transparent"; }}
              >
                <item.icon className="w-4 h-4 shrink-0" />
                <span className="flex-1">{item.label}</span>
              </button>
            );
          })}
        </nav>

        {/* Footer */}
        <div className="px-3 py-3 shrink-0" style={{ borderTop: "1px solid rgba(255,255,255,.08)" }}>
          <div className="flex items-center gap-2 px-1">
            <Clock className="w-3 h-3 text-zinc-400" />
            <span className="text-xs font-mono text-zinc-200">{currentTime}</span>
            <span className="ml-auto text-[10px] font-semibold"
              style={{ color: isConnected ? "#10b981" : "#52525b" }}>
              {isConnected ? "● LIVE" : "○ OFFLINE"}
            </span>
          </div>
        </div>
      </aside>

      {/* ── Content panel ──────────────────────────────────────────────── */}
      <div className="flex-1 flex flex-col overflow-hidden rounded-2xl"
        style={{ background: "#f5f5f7", boxShadow: "none" }}>

        {/* Topbar */}
        <div className="shrink-0 h-[52px] flex items-center px-5 gap-3 bg-white"
          style={{ borderBottom: "1px solid #e4e4e7", WebkitAppRegion: "drag" } as React.CSSProperties}>
          <h1 className="flex-1 text-[14px] font-semibold text-zinc-800">{TAB_TITLE[tab]}</h1>

          {error && (
            <div className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs"
              style={{ background: "#fef2f2", border: "1px solid #fecaca", color: "#dc2626", WebkitAppRegion: "no-drag" } as React.CSSProperties}>
              <AlertCircle className="w-3.5 h-3.5" />
              <span>{error}</span>
              <button onClick={reconnect} className="underline ml-1 cursor-pointer">Retry</button>
            </div>
          )}

          {/* File picker — hidden on Mail Queue and Log Source */}
          {tab !== "queue" && tab !== "logsource" && (
            <div className="flex items-center gap-1.5 h-8 px-2.5 rounded-lg border border-zinc-200 bg-white"
              style={{ WebkitAppRegion: "no-drag" } as React.CSSProperties}>
              <FolderOpen className="w-3.5 h-3.5 text-zinc-400 shrink-0" />
              <NativeSelect
                value={selectedFile || ""}
                onChange={e => { if (e.target.value) void refreshFile(e.target.value); }}
                className="!border-none !bg-transparent !shadow-none h-full pl-0 pr-6 text-xs text-zinc-600"
              >
                {availableFiles.map(f => <option key={f} value={f}>{f}</option>)}
              </NativeSelect>
            </div>
          )}

          {/* Refresh — hidden on tabs that have their own toolbar refresh */}
          {tab !== "queue" && tab !== "logsource" && (
            <button
              onClick={() => void refreshFile()}
              title="Refresh"
              className="h-8 w-8 flex items-center justify-center rounded-lg transition-colors cursor-pointer hover:bg-zinc-50"
              style={{ border: "1px solid #e4e4e7", background: "white", WebkitAppRegion: "no-drag" } as React.CSSProperties}
            >
              <RefreshCw
                className={`w-3.5 h-3.5 transition-colors ${isLoading ? "animate-spin" : ""}`}
                style={{ color: isLoading ? "#10b981" : "#71717a" }}
              />
            </button>
          )}
        </div>

        {/* Page content — lazy mounted, hidden with display:none when not active */}
        <main className="flex-1 min-h-0 overflow-hidden relative">

          {/* Overview */}
          <div className="absolute inset-0 overflow-hidden flex flex-col"
            style={{ display: tab === "overview" ? "flex" : "none" }}>
            <div className="flex flex-col h-full p-5 gap-4 overflow-hidden">
              <div className="grid grid-cols-6 gap-3 shrink-0">
                <MetricCard
                  title="Success Rate"
                  value={metrics?.deliverySuccessRate ?? 0}
                  icon={CheckCircle2}
                  color={
                    (metrics?.deliverySuccessRate ?? 0) >= 95 ? "green" :
                    (metrics?.deliverySuccessRate ?? 0) >= 80 ? "yellow" : "red"
                  }
                  suffix="%"
                />
                <MetricCard title="Sent"     value={metrics?.totalSent     ?? 0} icon={Mail}          color="green"  onClick={() => goToActivity("sent")} />
                <MetricCard title="Deferred" value={metrics?.totalDeferred ?? 0} icon={Clock}         color="yellow" onClick={() => goToActivity("deferred")} />
                <MetricCard title="Bounced"  value={metrics?.totalBounced  ?? 0} icon={AlertCircle}   color="red"    onClick={() => goToActivity("bounced")} />
                <MetricCard title="Errors"   value={metrics?.errorCount    ?? 0} icon={AlertCircle}   color="red"    onClick={() => goToDiag("errors")} />
                <MetricCard title="Warnings" value={metrics?.warningCount  ?? 0} icon={AlertTriangle} color="yellow" onClick={() => goToDiag("warnings")} />
              </div>
              {metrics && (
                <div className="grid grid-cols-2 gap-4 flex-1 min-h-0">
                  <TopDomainsTable data={metrics.topSenderDomains}    title="Top Sender Domains"    color="blue" />
                  <TopDomainsTable data={metrics.topRecipientDomains} title="Top Recipient Domains" color="green" />
                </div>
              )}
            </div>
          </div>

          {/* Volume & Delivery */}
          {mountedTabs.has("volume") && (
            <div className="absolute inset-0 overflow-hidden flex flex-col"
              style={{ display: tab === "volume" ? "flex" : "none" }}>
              <VolumeInsightsChart data={volumeInsights} sinceHours={sinceHours} />
            </div>
          )}

          {/* Delay Insights */}
          {mountedTabs.has("insights") && (
            <div className="absolute inset-0 overflow-hidden flex gap-4 p-4"
              style={{ display: tab === "insights" ? "flex" : "none" }}>
              {/* Left: Delay Insights chart */}
              <div className="flex-1 min-w-0 rounded-xl border border-zinc-100 bg-white overflow-y-auto">
                {delayInsights && <DelayInsightsChart delayInsights={delayInsights} />}
              </div>
              {/* Right: Retry Timeline */}
              <div className="w-72 shrink-0 rounded-xl border border-zinc-100 bg-white overflow-hidden flex flex-col">
                <div className="px-5 py-3.5 border-b border-zinc-100 shrink-0">
                  <h3 className="text-sm font-semibold text-zinc-800">Retry Timeline</h3>
                  <p className="text-xs mt-0.5 text-zinc-400">Delivery attempts per queue ID</p>
                </div>
                <div className="flex-1 min-h-0 overflow-hidden">
                  <RetryTimelinePanel sinceHours={sinceHours} />
                </div>
              </div>
            </div>
          )}

          {/* Qmgr */}
          {mountedTabs.has("qmgr") && (
            <div className="absolute inset-0 overflow-y-auto p-5"
              style={{ display: tab === "qmgr" ? "block" : "none" }}>
              {metrics && <QmgrActivityChart activeData={metrics.qmgrActiveHistory} removedData={metrics.qmgrRemovedHistory} />}
            </div>
          )}

          {/* Activity */}
          {mountedTabs.has("activity") && (
            <div className="absolute inset-0 overflow-hidden"
              style={{ display: tab === "activity" ? "block" : "none" }}>
              <ActivityTable refreshKey={refreshKey} initialStatus={activityFilter} filterSeq={activityFilterSeq} />
            </div>
          )}

          {/* Queue Summary */}
          {mountedTabs.has("queuesummary") && (
            <div className="absolute inset-0 overflow-hidden"
              style={{ display: tab === "queuesummary" ? "block" : "none" }}>
              <QueueSummaryTable sinceHours={sinceHours} refreshKey={refreshKey} />
            </div>
          )}

          {/* Mail Queue */}
          {mountedTabs.has("queue") && (
            <div className="absolute inset-0 overflow-hidden"
              style={{ display: tab === "queue" ? "block" : "none" }}>
              <MailQueueTable refreshKey={refreshKey} />
            </div>
          )}

          {/* Relay Insights */}
          {mountedTabs.has("relays") && (
            <div className="absolute inset-0 overflow-hidden flex flex-col"
              style={{ display: tab === "relays" ? "flex" : "none" }}>
              <RelayInsightsPanel sinceHours={sinceHours} refreshKey={refreshKey} />
            </div>
          )}

          {mountedTabs.has("errors") && (
            <div className="absolute inset-0 overflow-hidden flex gap-4 p-4"
              style={{ display: tab === "errors" ? "flex" : "none" }}>
              {/* Left: IP Reputation */}
              <div className="w-80 shrink-0 rounded-xl border border-zinc-100 bg-white flex flex-col overflow-hidden" style={{ position: "relative" }}>
                <div className="px-5 py-3.5 border-b border-zinc-100 shrink-0">
                  <h3 className="text-sm font-semibold text-zinc-800">IP Reputation</h3>
                  <p className="text-xs mt-0.5 text-zinc-400">Blocklist events for your sending IPs</p>
                </div>
                <div className="flex-1 overflow-y-auto p-4" style={{ position: "relative" }}>
                  <IPReputationPanel sinceHours={sinceHours} refreshKey={refreshKey} />
                </div>
              </div>
              {/* Right: Error log */}
              <div className="flex-1 min-w-0 rounded-xl border border-zinc-100 bg-white overflow-hidden">
                <ErrorLogViewer refreshKey={refreshKey} initialType={diagFilter} />
              </div>
            </div>
          )}

          {/* Raw Log */}
          {mountedTabs.has("rawlog") && (
            <div className="absolute inset-0 overflow-hidden"
              style={{ display: tab === "rawlog" ? "block" : "none" }}>
              <RawLogViewer refreshKey={refreshKey} selectedFile={selectedFile} />
            </div>
          )}

          {/* Log Source */}
          {mountedTabs.has("logsource") && (
            <div className="absolute inset-0 overflow-hidden"
              style={{ display: tab === "logsource" ? "block" : "none" }}>
              <LogSourcePanel onLoaded={() => { fetchAvailableFiles(); refreshFile("mail.log"); setTab("overview"); }} />
            </div>
          )}

        </main>
      </div>
    </div>
  );
}
