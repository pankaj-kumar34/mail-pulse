import { useState, useCallback, useEffect } from "react";
import {
  GetMetrics,
  GetDelayInsights,
  GetStatus,
  IngestFile,
  ListLogFiles,
  RefreshFile,
} from "../../bindings/github.com/pankajkumar/mail-pulse/logservice.js";

export function usePostfixLogs() {
  const [isConnected, setIsConnected] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshKey, setRefreshKey] = useState(0);
  const [selectedFile, setSelectedFile] = useState("mail.0.log");
  const [availableFiles, setAvailableFiles] = useState<string[]>([]);

  const ingest = useCallback(async () => {
    setIsLoading(true);
    try {
      const result = await IngestFile();
      const sel = result?.["selectedFile"] as string | undefined;
      if (sel) setSelectedFile(sel);
      setIsConnected(true);
      setError(null);
      setRefreshKey((k) => k + 1);
    } catch (e) {
      setIsConnected(false);
      setError("Failed to load log file.");
      console.error(e);
    } finally {
      setIsLoading(false);
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
    try {
      const result = await RefreshFile(filename ?? "");
      if (filename) setSelectedFile(filename);
      else {
        const sel = result?.["selectedFile"] as string | undefined;
        if (sel) setSelectedFile(sel);
      }
      setError(null);
      setRefreshKey((k) => k + 1);
    } catch (_) {
      setError("Failed to refresh from file.");
    } finally {
      setIsLoading(false);
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
    error,
    reconnect,
    refreshFile,
    refreshKey,
    selectedFile,
    availableFiles,
    fetchAvailableFiles,
  };
}

export { GetMetrics, GetDelayInsights };
