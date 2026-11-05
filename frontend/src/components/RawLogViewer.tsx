import { useState, useEffect, useRef, useCallback } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { Search, WrapText, AlignLeft, X } from "lucide-react";
import { EmptyState } from "./EmptyState";
import { LogService } from "../../bindings/github.com/pankajkumar/mail-pulse";
import { Input } from "@/components/ui/input";

interface Props {
  refreshKey: number;
  selectedFile: string | null;
}

const PAGE   = 200;
const LINE_H = 18;
const SEARCH_PAGE = 200; // search results per page
const DEBOUNCE_MS = 400;

interface SearchMatch { lineNum: number; text: string; }

function lineColor(line: string): string {
  const l = line.toLowerCase();
  if (l.includes("error"))                         return "#dc2626";
  if (l.includes("warning") || l.includes("warn")) return "#d97706";
  return "#374151";
}

function highlight(line: string, query: string) {
  if (!query) return line;
  const idx = line.toLowerCase().indexOf(query.toLowerCase());
  if (idx === -1) return line;
  return (<>
    {line.slice(0, idx)}
    <mark className="rounded px-0.5" style={{ background: "rgba(16,185,129,0.3)", color: "inherit" }}>
      {line.slice(idx, idx + query.length)}
    </mark>
    {line.slice(idx + query.length)}
  </>);
}

export function RawLogViewer({ refreshKey, selectedFile }: Props) {
  // ── Normal scroll state ───────────────────────────────────────────────────
  const [total, setTotal]       = useState(0);
  const [indexing, setIndexing] = useState(false);
  const [wrap, setWrap]         = useState(false);
  const [tick, setTick]         = useState(0);

  // ── Search state ──────────────────────────────────────────────────────────
  const [query, setQuery]             = useState("");
  const [debouncedQuery, setDebounced] = useState("");
  const [searching, setSearching]     = useState(false);
  const [searchMatches, setSearchMatches] = useState<SearchMatch[]>([]);
  const [searchTotal, setSearchTotal] = useState(0);
  const [searchOffset, setSearchOffset] = useState(0);

  const parentRef   = useRef<HTMLDivElement>(null);
  const fileRef     = useRef<string | null>(null);
  const cacheRef    = useRef<Map<number, string>>(new Map());
  const fetchingRef = useRef<Set<number>>(new Set());

  const isSearching = debouncedQuery.length > 0;

  // ── Index ─────────────────────────────────────────────────────────────────
  const indexFile = useCallback(async (filename: string) => {
    setIndexing(true);
    setTotal(0);
    fileRef.current = filename;
    cacheRef.current.clear();
    fetchingRef.current.clear();
    try {
      const res = await LogService.IndexFile(filename) as any;
      setTotal(Number(res.total ?? 0));
    } catch (e) { console.error(e); }
    finally { setIndexing(false); }
  }, []);

  useEffect(() => {
    if (!selectedFile) return;
    setQuery("");
    setDebounced("");
    indexFile(selectedFile);
  }, [selectedFile, refreshKey, indexFile]);

  // ── Debounce query → debouncedQuery ──────────────────────────────────────
  useEffect(() => {
    const id = setTimeout(() => setDebounced(query), DEBOUNCE_MS);
    return () => clearTimeout(id);
  }, [query]);

  // ── Backend search when debouncedQuery changes ────────────────────────────
  const runSearch = useCallback(async (filename: string, q: string, offset: number) => {
    if (!q) { setSearchMatches([]); setSearchTotal(0); return; }
    setSearching(true);
    try {
      const res = await LogService.SearchLines(filename, q, offset, SEARCH_PAGE) as any;
      const matches: SearchMatch[] = res.matches ?? [];
      const total: number = Number(res.total ?? 0);
      setSearchMatches(prev => offset === 0 ? matches : [...prev, ...matches]);
      setSearchTotal(total);
      setSearchOffset(offset + matches.length);
    } catch (e) { console.error(e); }
    finally { setSearching(false); }
  }, []);

  useEffect(() => {
    const filename = fileRef.current;
    if (!filename) return;
    setSearchOffset(0);
    setSearchMatches([]);
    if (debouncedQuery) runSearch(filename, debouncedQuery, 0);
    else { setSearchTotal(0); }
  }, [debouncedQuery, runSearch]);

  // ── Fetch pages for virtual scroll ───────────────────────────────────────
  const fetchPage = useCallback(async (pageStart: number) => {
    const filename = fileRef.current;
    if (!filename) return;
    if (fetchingRef.current.has(pageStart) || cacheRef.current.has(pageStart)) return;
    fetchingRef.current.add(pageStart);
    try {
      const res = await LogService.GetLines(filename, pageStart, PAGE) as any;
      const fetched: string[] = res.lines ?? [];
      fetched.forEach((l, i) => cacheRef.current.set(pageStart + i, l));
      setTick(t => t + 1);
    } catch (e) { console.error(e); }
    finally { fetchingRef.current.delete(pageStart); }
  }, []);

  // ── Live poll ─────────────────────────────────────────────────────────────
  useEffect(() => {
    const id = setInterval(async () => {
      const filename = fileRef.current;
      if (!filename) return;
      try {
        const res = await LogService.PollNewLines(filename) as any;
        const newCount: number = res.newCount ?? 0;
        const newTotal: number = Number(res.total ?? 0);
        const newLines: string[] = res.lines ?? [];
        if (newCount > 0) {
          newLines.forEach((l, i) => cacheRef.current.set(newTotal - newCount + i, l));
          setTotal(newTotal);
        }
      } catch (e) { console.error(e); }
    }, 3000);
    return () => clearInterval(id);
  }, []);

  // ── Virtual scroll list ───────────────────────────────────────────────────
  const scrollVirtualizer = useVirtualizer({
    count: isSearching ? 0 : total,
    getScrollElement: () => parentRef.current,
    estimateSize: () => LINE_H,
    overscan: 5,
  });

  const scrollItems = scrollVirtualizer.getVirtualItems();

  useEffect(() => {
    if (isSearching) return;
    const pages = new Set(
      scrollItems
        .filter(v => !cacheRef.current.has(v.index))
        .map(v => Math.floor(v.index / PAGE) * PAGE)
    );
    pages.forEach(fetchPage);
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [scrollItems, tick, isSearching, fetchPage]);

  // ── Load more search results when near end ────────────────────────────────
  const searchListRef = useRef<HTMLDivElement>(null);
  const handleSearchScroll = useCallback(() => {
    const el = searchListRef.current;
    if (!el || searching) return;
    if (el.scrollTop + el.clientHeight >= el.scrollHeight - 200) {
      const filename = fileRef.current;
      if (filename && searchOffset < searchTotal) {
        runSearch(filename, debouncedQuery, searchOffset);
      }
    }
  }, [searching, searchOffset, searchTotal, debouncedQuery, runSearch]);

  const clearSearch = () => { setQuery(""); setDebounced(""); };

  // ── Render ────────────────────────────────────────────────────────────────
  return (
    <div className="flex flex-col h-full bg-white">
      {/* Toolbar */}
      <div className="shrink-0 flex items-center gap-2 px-5 py-3 border-b border-zinc-100">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-zinc-400 pointer-events-none" />
          <Input
            placeholder="Search full file…"
            value={query}
            onChange={e => setQuery(e.target.value)}
            className="pl-10 pr-8"
          />
          {query && (
            <button onClick={clearSearch}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-zinc-400 hover:text-zinc-600 cursor-pointer">
              <X className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
        <button onClick={() => setWrap(w => !w)} title={wrap ? "No wrap" : "Wrap lines"}
          className={`h-9 w-9 flex items-center justify-center rounded-lg border transition-colors cursor-pointer ${wrap ? "bg-emerald-50 border-emerald-200" : "hover:bg-zinc-50 border-zinc-200"}`}>
          {wrap ? <WrapText className="w-3.5 h-3.5" style={{ color: "#10b981" }} />
                : <AlignLeft className="w-3.5 h-3.5 text-zinc-400" />}
        </button>
      </div>

      {/* Content: search results OR virtual scroll */}
      {isSearching ? (
        // ── Search results ────────────────────────────────────────────────
        <div ref={searchListRef} className="flex-1 overflow-auto px-5 py-2"
          style={{ background: "#fafafa" }} onScroll={handleSearchScroll}>
          {searching && searchMatches.length === 0 ? (
            <p className="text-zinc-400 font-sans text-sm text-center py-16">Searching…</p>
          ) : searchMatches.length === 0 ? (
            <p className="text-zinc-400 font-sans text-sm text-center py-16">No matches</p>
          ) : (
            <>
              {searchMatches.map((m, i) => (
                <div key={i}
                  className={`select-text text-[11px] leading-[1.6] hover:bg-zinc-100 rounded px-1 -mx-1 ${wrap ? "whitespace-pre-wrap break-all" : "whitespace-nowrap overflow-hidden"}`}
                  style={{ color: lineColor(m.text) }}>
                  <span className="text-zinc-300 select-none mr-3 tabular-nums">
                    {(m.lineNum + 1).toLocaleString()}
                  </span>
                  {highlight(m.text, debouncedQuery)}
                </div>
              ))}
              {searching && (
                <p className="text-zinc-400 font-sans text-xs text-center py-3 animate-pulse">Loading more…</p>
              )}
              {searchOffset >= searchTotal && searchTotal > 0 && (
                <p className="text-zinc-400 font-sans text-xs text-center py-3">
                  All {searchTotal.toLocaleString()} matches loaded
                </p>
              )}
            </>
          )}
        </div>
      ) : (
        // ── Virtual scroll ────────────────────────────────────────────────
        <div ref={parentRef} className="flex-1 overflow-auto" style={{ background: "#fafafa" }}>
          {indexing ? (
            <p className="text-zinc-400 font-sans text-sm text-center py-16">Indexing…</p>
          ) : total === 0 ? (
            <EmptyState icon="log" title="No lines" subtitle="Load a log file to view its contents" fill />
          ) : (
            <div style={{ height: scrollVirtualizer.getTotalSize(), position: "relative" }}>
              {scrollItems.map(vItem => {
                const line = cacheRef.current.get(vItem.index);
                return (
                  <div key={vItem.key} data-index={vItem.index}
                    ref={scrollVirtualizer.measureElement}
                    style={{ position: "absolute", top: 0, left: 0, width: "100%", transform: `translateY(${vItem.start}px)` }}>
                    <div
                      className={`select-text text-[11px] leading-[1.6] px-5 hover:bg-zinc-100 ${wrap ? "whitespace-pre-wrap break-all" : "whitespace-nowrap overflow-hidden"}`}
                      style={{ color: line ? lineColor(line) : "#d4d4d8" }}>
                      {line ?? <span className="opacity-20 font-sans">—</span>}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* Footer */}
      <div className="shrink-0 flex items-center justify-between px-5 py-3 border-t border-zinc-100 font-sans">
        <span className="text-xs text-zinc-400 flex items-center gap-2">
          {isSearching ? (
            <>
              <Search className="w-3 h-3" />
              {searching && searchMatches.length === 0
                ? "Searching…"
                : `${searchTotal.toLocaleString()} matches`}
            </>
          ) : (
            <>
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse inline-block" />
              {indexing ? "Indexing…" : `${total.toLocaleString()} lines`}
            </>
          )}
        </span>
        <span className="text-xs text-zinc-400">{selectedFile}</span>
      </div>
    </div>
  );
}
