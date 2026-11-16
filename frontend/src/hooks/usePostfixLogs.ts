import { useState, useCallback, useEffect } from "react";
import {
  GetStatus,
  ListLogFiles,
  StartIngest,
  GetIngestStatus,
  RefreshFile,
} from "../../bindings/github.com/pankajkumar/mail-pulse/logservice.js";
import { GetMetrics, GetDelayInsights } from "../../bindings/github.com/pankajkumar/mail-pulse/logservice.js";

export { GetMetrics, GetDelayInsights };

const POLL_MS = 200;

/** Poll GetIngestStatus every POLL_MS until done or error. Returns the final error string (empty = success). */
async function pollUntilDone(
  onProgress: (pct: number, msg: string) => void
): Promise<string> {
  return new Promise((resolve) => {
    const id = setInterval(async () => {
      try {
        const s = (await GetIngestStatus()) as any;
        if (s?.pct !== undefined) {
          onProgress(Number(s.pct), String(s.msg ?? ""));
        }
        if (s?.done) {
          clearInterval(id);
          resolve(String(s.error ?? ""));
        }
      } catch (_) {
        // transient IPC error — keep polling
      }
    }, POLL_MS);
  });
}

export function usePostfixLogs() {
  const [isConnected, setIsConnected] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);
  const [selectedFile, setSelectedFile] = useState("mail.0.log");
  const [availableFiles, setAvailableFiles] = useState<string[]>([]);
  const [ingestProgress, setIngestProgress] = useState<{ pct: number; msg: string } | null>(null);

  const ingest = useCallback(async () => {
    setIsLoading(true);
    setIngestProgress({ pct: 0, msg: "Starting…" });
    try {
      const result = await StartIngest() as any;
      const sel = result?.["selectedFile"] as string | undefined;
      if (sel) setSelectedFile(sel);

      const errMsg = await pollUntilDone((pct, msg) =>
        setIngestProgress({ pct, msg })
      );

      if (errMsg && errMsg !== "") {
        setIsConnected(false);
        setError("Failed to load log file.");
      } else {
        setIsConnected(true);
        setError(null);
        setRefreshKey((k) => k + 1);
      }
    } catch (e) {
      setIsConnected(false);
      setError("Failed to load log file.");
      console.error(e);
    } finally {
      setIsLoading(false);
      setIngestProgress(null);
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const status = await GetStatus();
        if (!cancelled && status?.["selectedFile"]) {
          setSelectedFile(status["selectedFile"] as string);
        }
      } catch (_) {
        // ignore
      }
      if (!cancelled) await ingest();
    })();
    return () => { cancelled = true; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const reconnect = useCallback(() => { void ingest(); }, [ingest]);

  const refreshFile = useCallback(async (filename?: string) => {
    setIsLoading(true);
    setIngestProgress({ pct: 0, msg: "Starting…" });
    try {
      // RefreshFile resets + starts async ingest, returns immediately
      const result = await RefreshFile(filename ?? "") as any;
      if (filename) setSelectedFile(filename);
      else {
        const sel = result?.["selectedFile"] as string | undefined;
        if (sel) setSelectedFile(sel);
      }

      const errMsg = await pollUntilDone((pct, msg) =>
        setIngestProgress({ pct, msg })
      );

      if (errMsg && errMsg !== "") {
        setError("Failed to refresh from file.");
      } else {
        setIsConnected(true);
        setError(null);
        setRefreshKey((k) => k + 1);
      }
    } catch (_) {
      setError("Failed to refresh from file.");
    } finally {
      setIsLoading(false);
      setIngestProgress(null);
    }
  }, []);

  const fetchAvailableFiles = useCallback(async () => {
    try {
      const files = await ListLogFiles();
      setAvailableFiles((files ?? []) as string[]);
    } catch (_) {
      // silently ignore
    }
  }, []);

  return {
    isConnected,
    isLoading,
    ingestProgress,
    error,
    reconnect,
    refreshFile,
    refreshKey,
    selectedFile,
    availableFiles,
    fetchAvailableFiles,
  };
}
