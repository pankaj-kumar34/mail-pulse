import { useEffect, useRef, useState } from "react";
import { COSService } from "../../bindings/github.com/pankajkumar/mail-pulse";
import { Input } from "@/components/ui/input";

// ── Types ─────────────────────────────────────────────────────────────────────
interface COSBucket  { name: string; bucket: string; endpoint: string; }
interface COSSource  { name: string; api_key: string; instance_id: string; buckets: COSBucket[]; }
interface COSConfig  { sources: COSSource[]; }
interface DownloadFile   { name: string; status: string; message?: string; }
interface DownloadStatus { phase: string; files: DownloadFile[]; message?: string; }

const emptySource  = (): COSSource  => ({ name: "", api_key: "", instance_id: "", buckets: [] });
const emptyBucket  = (): COSBucket  => ({ name: "", bucket: "", endpoint: "" });
const emptyConfig  = (): COSConfig  => ({ sources: [] });

// ── Tiny SVG icon ─────────────────────────────────────────────────────────────
function Ico({ d, size = 14, cls = "" }: { d: string; size?: number; cls?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none"
      stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round"
      className={cls}>
      <path d={d} />
    </svg>
  );
}
const I = {
  plus:     "M12 5v14M5 12h14",
  trash:    "M3 6h18M8 6V4h8v2M19 6l-1 14H6L5 6",
  check:    "M20 6L9 17l-5-5",
  x:        "M18 6 6 18M6 6l12 12",
  edit:     "M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z",
  cloud:    "M18 10h-1.26A8 8 0 1 0 9 20h9a5 5 0 0 0 0-10z",
  calendar: "M8 2v4M16 2v4M3 10h18M5 4h14a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2z",
  file:     "M13 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9zM13 2v7h7",
  refresh:  "M23 4v6h-6M1 20v-6h6M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15",
  download: "M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4M7 10l5 5 5-5M12 15V3",
  settings: "M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6zm6.93-3a6.97 6.97 0 0 0-.12-1.22l2.64-2.05-1-1.73-3.04 1.15A7 7 0 0 0 15 6.6l-.46-3.13h-2l-.46 3.13A7 7 0 0 0 9.59 8.15L6.55 7 5.55 8.73l2.64 2.05A6.97 6.97 0 0 0 8.07 12c0 .42.04.83.12 1.22L5.55 15.27l1 1.73 3.04-1.15a7 7 0 0 0 2.41 1.55l.46 3.13h2l.46-3.13a7 7 0 0 0 2.41-1.55l3.04 1.15 1-1.73-2.64-2.05c.08-.39.12-.8.12-1.22z",
};

// ── Source editor (one card per source) ───────────────────────────────────────
function SourceCard({
  src, selected, onSelect, onChange, onDelete,
}: {
  src: COSSource;
  selected: boolean;
  onSelect: () => void;
  onChange: (s: COSSource) => void;
  onDelete: () => void;
}) {
  const [open, setOpen] = useState(!src.name); // new cards start open

  const updField = (f: keyof COSSource, v: string) => onChange({ ...src, [f]: v });
  const addBucket   = () => onChange({ ...src, buckets: [...src.buckets, emptyBucket()] });
  const removeBucket = (i: number) => onChange({ ...src, buckets: src.buckets.filter((_, j) => j !== i) });
  const updBucket = (i: number, f: keyof COSBucket, v: string) =>
    onChange({ ...src, buckets: src.buckets.map((b, j) => j === i ? { ...b, [f]: v } : b) });

  return (
    <div className={`rounded-xl border bg-white overflow-hidden transition-colors ${
      selected ? "border-emerald-300 ring-1 ring-emerald-200" : "border-zinc-200"
    }`}>
      {/* Header row */}
      <div className={`flex items-center gap-3 px-4 py-3 border-b transition-colors ${
        selected ? "bg-emerald-50 border-emerald-100" : "bg-zinc-50 border-zinc-100"
      }`}>
        {/* Select indicator */}
        <button onClick={onSelect}
          className={`w-4 h-4 rounded-full border-2 flex items-center justify-center shrink-0 transition-colors ${
            selected ? "border-emerald-600 bg-emerald-600" : "border-zinc-300 hover:border-emerald-400"
          }`}>
          {selected && <div className="w-1.5 h-1.5 rounded-full bg-white" />}
        </button>

        <button onClick={() => setOpen(o => !o)} className="flex items-center gap-2 flex-1 text-left">
          <span className={`text-sm font-semibold ${selected ? "text-emerald-800" : "text-zinc-800"}`}>
            {src.name || "New Source"}
          </span>
          <span className="text-xs text-zinc-400">{src.buckets.length} bucket{src.buckets.length !== 1 ? "s" : ""}</span>
          <span className="ml-auto text-zinc-400 text-xs">{open ? "▲" : "▼"}</span>
        </button>
        <button onClick={onDelete}
          className="p-1.5 rounded-md text-zinc-400 hover:text-rose-500 hover:bg-rose-50 transition-colors">
          <Ico d={I.trash} size={13} />
        </button>
      </div>

      {/* Body */}
      {open && (
        <div className="p-4 flex flex-col gap-4">
          {/* Source name */}
          <div>
            <label className="block text-xs font-medium text-zinc-500 mb-1.5">Source Name</label>
            <Input placeholder="e.g. Stage, Dallas, Frankfurt…" value={src.name}
              onChange={e => updField("name", e.target.value)} />
          </div>

          {/* Credentials */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-zinc-500 mb-1.5">API Key</label>
              <Input type="password" placeholder="••••••••" value={src.api_key}
                onChange={e => updField("api_key", e.target.value)} />
            </div>
            <div>
              <label className="block text-xs font-medium text-zinc-500 mb-1.5">Instance ID</label>
              <Input type="password" placeholder="••••••••" value={src.instance_id}
                onChange={e => updField("instance_id", e.target.value)} />
            </div>
          </div>

          {/* Buckets */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="text-xs font-medium text-zinc-500">Buckets</span>
              <button onClick={addBucket}
                className="flex items-center gap-1 text-xs text-emerald-600 hover:text-emerald-700 font-medium transition-colors">
                <Ico d={I.plus} size={12} /> Add Bucket
              </button>
            </div>
            {src.buckets.length === 0 && (
              <p className="text-xs text-zinc-400 py-1">No buckets yet.</p>
            )}
            <div className="flex flex-col gap-2">
              {src.buckets.map((b, i) => (
                <div key={i} className="grid grid-cols-[1fr_1.5fr_2fr_auto] gap-2 items-center">
                  <Input placeholder="Label (e.g. US)" value={b.name}
                    onChange={e => updBucket(i, "name", e.target.value)} />
                  <Input placeholder="Bucket name" value={b.bucket}
                    onChange={e => updBucket(i, "bucket", e.target.value)} />
                  <Input placeholder="s3.us-south.cloud-object-storage.appdomain.cloud" value={b.endpoint}
                    onChange={e => updBucket(i, "endpoint", e.target.value)} />
                  <button onClick={() => removeBucket(i)}
                    className="p-1.5 rounded-md text-zinc-400 hover:text-rose-500 hover:bg-rose-50 transition-colors">
                    <Ico d={I.trash} size={13} />
                  </button>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// ── Settings panel ────────────────────────────────────────────────────────────
function SettingsSection({ onSaved, selectedSource, onSelectSource }: {
  onSaved: () => void;
  selectedSource: string;
  onSelectSource: (name: string) => void;
}) {
  const [cfg, setCfg]     = useState<COSConfig>(emptyConfig());
  const [saving, setSaving] = useState(false);
  const [error, setError]   = useState("");

  useEffect(() => {
    COSService.GetCOSConfig().then((c: any) => {
      const loaded = c?.sources ? c : emptyConfig();
      setCfg(loaded);
      // auto-select first source if none selected
      if (!selectedSource && loaded.sources.length > 0) {
        onSelectSource(loaded.sources[0].name);
      }
    });
  }, []);

  const addSource    = () => setCfg(p => ({ sources: [...p.sources, emptySource()] }));
  const removeSource = (i: number) => setCfg(p => ({ sources: p.sources.filter((_, j) => j !== i) }));
  const updateSource = (i: number, s: COSSource) =>
    setCfg(p => ({ sources: p.sources.map((x, j) => j === i ? s : x) }));

  const save = async () => {
    setSaving(true); setError("");
    try {
      await COSService.SaveCOSConfig(cfg);
      onSaved();
    } catch (e: any) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="rounded-xl border border-zinc-100 bg-white">
      {/* Header */}
      <div className="flex items-center gap-3 px-5 py-4 border-b border-zinc-100">
        <div className="p-1.5 rounded-lg bg-zinc-100">
          <Ico d={I.settings} size={15} cls="text-zinc-600" />
        </div>
        <div className="flex-1">
          <div className="text-sm font-semibold text-zinc-800">COS Sources</div>
          <div className="text-xs text-zinc-400 mt-0.5">Click a source to select it for browsing</div>
        </div>
        <button onClick={addSource}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-emerald-200 text-xs font-medium text-emerald-700 bg-emerald-50 hover:bg-emerald-100 transition-colors">
          <Ico d={I.plus} size={12} /> Add Source
        </button>
      </div>

      <div className="p-5 flex flex-col gap-3">
        {cfg.sources.length === 0 && (
          <p className="text-sm text-zinc-400 py-2">No sources configured. Click "Add Source" to get started.</p>
        )}
        {cfg.sources.map((src, i) => (
          <SourceCard key={i} src={src}
            selected={src.name === selectedSource}
            onSelect={() => onSelectSource(src.name)}
            onChange={s => updateSource(i, s)}
            onDelete={() => removeSource(i)} />
        ))}

        {cfg.sources.length > 0 && (
          <div className="flex items-center gap-3 pt-1">
            <button onClick={save} disabled={saving}
              className="px-4 py-2 rounded-lg bg-emerald-600 text-white text-sm font-medium hover:bg-emerald-500 disabled:opacity-50 transition-colors">
              {saving ? "Saving…" : "Save All"}
            </button>
            {error && <span className="text-xs text-rose-600">{error}</span>}
          </div>
        )}
      </div>
    </div>
  );
}

// ── File status icon ──────────────────────────────────────────────────────────
function FileStatusIcon({ status }: { status: string }) {
  if (status === "done")        return <Ico d={I.check} size={13} cls="text-emerald-600" />;
  if (status === "error")       return <Ico d={I.x} size={13} cls="text-rose-600" />;
  if (status === "downloading") return (
    <svg width={13} height={13} viewBox="0 0 24 24" fill="none" stroke="#3b82f6"
      strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="animate-spin">
      <path d="M21 12a9 9 0 1 1-6.219-8.56" />
    </svg>
  );
  return <div style={{ width: 8, height: 8, borderRadius: "50%", background: "#d4d4d8", marginLeft: 2 }} />;
}

// ── Browse section ────────────────────────────────────────────────────────────
function BrowseSection({ configVersion, selectedSource, onLoaded }: {
  configVersion: number;
  selectedSource: string;
  onLoaded: () => void;
}) {
  const [sources, setSources]             = useState<COSSource[]>([]);
  const [selectedBucket, setSelectedBucket] = useState("");
  const [dates, setDates]               = useState<string[]>([]);
  const [loadingDates, setLoadingDates] = useState(false);
  const [selectedDate, setSelectedDate] = useState("");
  const [files, setFiles]               = useState<string[]>([]);
  const [loadingFiles, setLoadingFiles] = useState(false);
  const [selectedFiles, setSelectedFiles] = useState<Set<string>>(new Set());
  const [status, setStatus]             = useState<DownloadStatus | null>(null);
  const [downloading, setDownloading]   = useState(false);
  const [dlError, setDlError]           = useState("");
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  useEffect(() => {
    COSService.GetCOSConfig().then((c: any) => {
      const srcs: COSSource[] = c?.sources ?? [];
      setSources(srcs);
    });
  }, [configVersion]);

  // When selected source changes, reset bucket selection
  useEffect(() => {
    const src = sources.find(s => s.name === selectedSource);
    setSelectedBucket(src?.buckets?.[0]?.name ?? "");
    setDates([]); setSelectedDate(""); setFiles([]); setSelectedFiles(new Set());
  }, [selectedSource, sources]);

  const currentSource = sources.find(s => s.name === selectedSource);
  const buckets = currentSource?.buckets ?? [];

  const loadDates = async () => {
    if (!selectedSource || !selectedBucket) return;
    setLoadingDates(true); setDates([]); setSelectedDate(""); setFiles([]); setSelectedFiles(new Set());
    try {
      const d: any = await COSService.ListCOSDates(selectedSource, selectedBucket);
      setDates(d ?? []);
    } catch (e: any) { setDlError(String(e)); }
    finally { setLoadingDates(false); }
  };

  const loadFiles = async (date: string) => {
    setSelectedDate(date); setFiles([]); setSelectedFiles(new Set()); setLoadingFiles(true);
    try {
      const f: any = await COSService.ListCOSFiles(selectedSource, selectedBucket, date);
      const list: string[] = f ?? [];
      setFiles(list);
      setSelectedFiles(new Set(list));
    } catch (e: any) { setDlError(String(e)); }
    finally { setLoadingFiles(false); }
  };

  const toggleFile = (key: string) => setSelectedFiles(prev => {
    const next = new Set(prev);
    if (next.has(key)) next.delete(key); else next.add(key);
    return next;
  });

  const startDownload = async () => {
    if (!selectedSource || !selectedBucket || !selectedDate || selectedFiles.size === 0) return;
    setDownloading(true); setDlError(""); setStatus(null);
    const keys = Array.from(selectedFiles);

    pollRef.current = setInterval(async () => {
      const s: any = await COSService.GetDownloadStatus();
      setStatus(s);
      if (s?.phase === "done") {
        clearInterval(pollRef.current!);
        setDownloading(false);
        setTimeout(() => onLoaded(), 2000);
      }
      if (s?.phase === "error") {
        clearInterval(pollRef.current!);
        setDownloading(false);
        setDlError(s.message ?? "Unknown error");
      }
    }, 400);

    try {
      await COSService.DownloadLogs(selectedSource, selectedBucket, selectedDate, keys);
    } catch (e: any) {
      clearInterval(pollRef.current!);
      setDownloading(false);
      setDlError(String(e));
    }
  };

  useEffect(() => () => { if (pollRef.current) clearInterval(pollRef.current); }, []);

  const phaseLabel: Record<string, string> = {
    checking: "Checking cache…", downloading: "Downloading…",
    merging: "Merging logs…", loading: "Loading into Mail Pulse…",
    done: "Done!", error: "Error",
  };

  if (sources.length === 0) return (
    <div className="rounded-xl border border-zinc-100 bg-white">
      <div className="flex items-center gap-3 px-5 py-4 border-b border-zinc-100">
        <div className="p-1.5 rounded-lg bg-zinc-100"><Ico d={I.cloud} size={15} cls="text-zinc-600" /></div>
        <div><div className="text-sm font-semibold text-zinc-800">Browse Logs</div>
          <div className="text-xs text-zinc-400 mt-0.5">Select a source and date to download</div></div>
      </div>
      <p className="px-5 py-4 text-sm text-zinc-400">Configure a COS source above first.</p>
    </div>
  );

  return (
    <div className="rounded-xl border border-zinc-100 bg-white">
      {/* Header — shows active source name */}
      <div className="flex items-center gap-3 px-5 py-4 border-b border-zinc-100">
        <div className="p-1.5 rounded-lg bg-zinc-100"><Ico d={I.cloud} size={15} cls="text-zinc-600" /></div>
        <div>
          <div className="text-sm font-semibold text-zinc-800">Browse Logs</div>
          <div className="text-xs text-zinc-400 mt-0.5">
            {selectedSource ? <>Source: <span className="text-emerald-600 font-medium">{selectedSource}</span></> : "Select a source above"}
          </div>
        </div>
      </div>

      <div className="p-5 flex flex-col gap-4">

        {/* ── Bucket row ── */}
        {buckets.length > 0 && (
          <div className="flex items-center gap-1.5 flex-wrap">
            {buckets.map(b => (
                <button key={b.name}
                  onClick={() => { setSelectedBucket(b.name); setDates([]); setSelectedDate(""); setFiles([]); }}
                  className={`px-3 h-8 rounded-lg text-xs font-medium border transition-colors ${
                    selectedBucket === b.name
                      ? "bg-emerald-600 text-white border-emerald-600"
                      : "bg-white text-zinc-600 border-zinc-200 hover:border-zinc-400 hover:text-zinc-800"
                  }`}>
                  {b.name}
                </button>
              ))}
              <button onClick={loadDates} disabled={loadingDates}
                className="flex items-center gap-1.5 px-3 h-8 rounded-lg text-xs font-medium bg-emerald-600 text-white hover:bg-emerald-500 disabled:opacity-50 transition-colors ml-auto">
                <Ico d={loadingDates ? I.refresh : I.calendar} size={12} cls={loadingDates ? "animate-spin" : ""} />
                {loadingDates ? "Loading…" : "Browse Dates"}
              </button>
          </div>
        )}

        {/* Date pills */}
        {dates.length > 0 && (
          <div className="flex flex-wrap gap-1.5">
            {dates.map(d => (
                <button key={d} onClick={() => loadFiles(d)}
                  className={`px-2.5 py-1 rounded-md text-xs font-medium border transition-colors ${
                    selectedDate === d
                      ? "bg-emerald-50 text-emerald-700 border-emerald-300"
                      : "bg-white text-zinc-500 border-zinc-200 hover:border-zinc-300 hover:text-zinc-700"
                  }`}>
                  {d}
                </button>
              ))}
          </div>
        )}

        {/* File list */}
        {loadingFiles && <p className="text-sm text-zinc-400">Loading files…</p>}
        {files.length > 0 && (
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <span className="text-xs font-medium text-zinc-500">Pod Files — {selectedDate}</span>
              <div className="flex gap-2 text-xs text-zinc-500">
                <button onClick={() => setSelectedFiles(new Set(files))} className="hover:text-zinc-800">All</button>
                <span className="text-zinc-300">·</span>
                <button onClick={() => setSelectedFiles(new Set())} className="hover:text-zinc-800">None</button>
              </div>
            </div>
            <div className="flex flex-col gap-1">
              {files.map(key => {
                const base = key.split("/").pop()!;
                const checked = selectedFiles.has(key);
                return (
                  <label key={key} className={`flex items-center gap-3 px-3 py-2 rounded-lg transition-colors ${checked ? "bg-zinc-50" : "hover:bg-zinc-50"}`}>
                    <input type="checkbox" checked={checked} onChange={() => toggleFile(key)}
                      className="rounded border-zinc-300" />
                    <Ico d={I.file} size={13} cls="text-zinc-400" />
                    <span className="text-sm font-mono text-zinc-700">{base}</span>
                  </label>
                );
              })}
            </div>
          </div>
        )}

        {/* Download */}
        {files.length > 0 && (
          <div className="flex items-center gap-4">
            <button onClick={startDownload} disabled={downloading || selectedFiles.size === 0}
              className="flex items-center gap-2 px-4 py-2 rounded-lg bg-emerald-600 text-white text-sm font-medium hover:bg-emerald-500 disabled:opacity-50 transition-colors">
              <Ico d={I.download} size={14} cls="text-white" />
              {downloading ? "Downloading…" : `Download ${selectedFiles.size} file${selectedFiles.size !== 1 ? "s" : ""}`}
            </button>
            {dlError && <span className="text-xs text-rose-600">{dlError}</span>}
          </div>
        )}

        {/* Progress */}
        {status && status.phase !== "idle" && (
          <div className="rounded-xl border border-zinc-100 bg-zinc-50 p-4 flex flex-col gap-3">
            <div className="flex items-center gap-2">
              {status.phase === "done"  ? <Ico d={I.check} size={14} cls="text-emerald-600" />
               : status.phase === "error" ? <Ico d={I.x} size={14} cls="text-rose-600" />
               : (
                <svg width={14} height={14} viewBox="0 0 24 24" fill="none" stroke="#3b82f6"
                  strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" className="animate-spin">
                  <path d="M21 12a9 9 0 1 1-6.219-8.56" />
                </svg>
              )}
              <span className={`text-sm font-medium ${status.phase === "done" ? "text-emerald-700" : status.phase === "error" ? "text-rose-700" : "text-zinc-700"}`}>
                {status.message || phaseLabel[status.phase] || status.phase}
              </span>
            </div>
            {status.files?.length > 0 && (
              <div className="flex flex-col gap-1.5">
                {status.files.map((f, i) => (
                  <div key={i} className="flex items-center gap-2">
                    <FileStatusIcon status={f.status} />
                    <span className="text-xs font-mono text-zinc-600">{f.name}</span>
                    {f.message && <span className="text-xs text-zinc-400">({f.message})</span>}
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// ── Main ──────────────────────────────────────────────────────────────────────
export default function LogSourcePanel({ onLoaded }: { onLoaded: () => void }) {
  const [configVersion, setConfigVersion] = useState(0);
  const [selectedSource, setSelectedSource] = useState("");
  return (
    <div className="flex flex-col gap-5 p-5 overflow-auto h-full">
      <SettingsSection
        onSaved={() => setConfigVersion(v => v + 1)}
        selectedSource={selectedSource}
        onSelectSource={setSelectedSource}
      />
      <BrowseSection configVersion={configVersion} selectedSource={selectedSource} onLoaded={onLoaded} />
    </div>
  );
}
