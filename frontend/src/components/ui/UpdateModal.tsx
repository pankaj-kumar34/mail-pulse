import React, { useCallback, useEffect, useState } from "react";
import {
  Download,
  RefreshCw,
  CheckCircle2,
  ExternalLink,
  Sparkles,
  ShieldCheck,
  AlertTriangle,
  X,
  ArrowRight,
  RotateCcw,
  Package,
} from "lucide-react";
import { Events } from "@wailsio/runtime";
import {
  UpdaterService,
  type AppVersionInfo,
  type UpdateCheckResult,
  type UpdateProgress,
} from "../../../bindings/github.com/pankajkumar/mail-pulse";
import { Button } from "./button";
import { formatBytes, getErrorMessage } from "../../lib/utils";

interface UpdateModalProps {
  isOpen: boolean;
  onClose: () => void;
  versionInfo: AppVersionInfo | null;
  initialCheckResult: UpdateCheckResult | null;
  onUpdateCheckComplete?: (result: UpdateCheckResult) => void;
}

export const UpdateModal: React.FC<UpdateModalProps> = ({
  isOpen,
  onClose,
  versionInfo,
  initialCheckResult,
  onUpdateCheckComplete,
}) => {
  const [checking, setChecking] = useState(false);
  const [localCheckResult, setLocalCheckResult] =
    useState<UpdateCheckResult | null>(null);
  const checkResult = localCheckResult ?? initialCheckResult;
  const [checkError, setCheckError] = useState<string | null>(null);
  const [installing, setInstalling] = useState(false);
  const [restarting, setRestarting] = useState(false);
  const [progress, setProgress] = useState<UpdateProgress | null>(null);
  const [installError, setInstallError] = useState<string | null>(null);

  const runCheck = useCallback(async () => {
    setChecking(true);
    setCheckError(null);
    setInstallError(null);
    setProgress(null);
    try {
      const res = await UpdaterService.CheckForUpdate();
      if (res) {
        setLocalCheckResult(res);
        if (onUpdateCheckComplete) {
          onUpdateCheckComplete(res);
        }
      }
    } catch (err: unknown) {
      setCheckError(getErrorMessage(err) || "Could not check GitHub Releases");
    } finally {
      setChecking(false);
    }
  }, [onUpdateCheckComplete]);

  useEffect(() => {
    if (!isOpen) return;

    const cancelProgress = Events.On("updater:progress", (ev: unknown) => {
      const raw = ev as { data?: UpdateProgress } | UpdateProgress | null;
      const data =
        raw && typeof raw === "object" && "data" in raw
          ? raw.data
          : (raw as UpdateProgress | null);
      if (data && data.stage) {
        setProgress(data);
        if (data.stage === "error") {
          setInstallError(data.message || "Update failed");
          setInstalling(false);
        } else if (data.stage === "ready") {
          setInstalling(false);
        }
      }
    });

    let checkTimer: ReturnType<typeof setTimeout> | undefined;
    if (!checkResult && !checking && !checkError) {
      checkTimer = setTimeout(() => {
        void runCheck();
      }, 0);
    }

    return () => {
      if (checkTimer !== undefined) clearTimeout(checkTimer);
      cancelProgress();
    };
  }, [isOpen, checkResult, checking, checkError, runCheck]);

  useEffect(() => {
    if (!isOpen) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !installing && !restarting) {
        e.preventDefault();
        onClose();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, installing, restarting, onClose]);

  const handleInstallUpdate = async () => {
    if (!checkResult || !checkResult.assetUrl) return;
    setInstalling(true);
    setInstallError(null);
    setProgress({
      stage: "downloading",
      percent: 0,
      downloadedBytes: 0,
      totalBytes: checkResult.assetSize || 0,
      message: `Downloading ${checkResult.assetName || "update"}...`,
    });

    try {
      await UpdaterService.DownloadAndInstallUpdate(
        checkResult.assetUrl,
        checkResult.assetName,
        checkResult.checksumUrl || "",
      );
      setProgress({
        stage: "ready",
        percent: 100,
        downloadedBytes: checkResult.assetSize || 0,
        totalBytes: checkResult.assetSize || 0,
        message: "Update installed! Restart Mail Pulse to apply.",
      });
    } catch (err: unknown) {
      setInstallError(getErrorMessage(err) || "Failed to install update");
    } finally {
      setInstalling(false);
    }
  };

  const handleRestartNow = async () => {
    setRestarting(true);
    try {
      await UpdaterService.RestartApp();
    } catch (err: unknown) {
      setRestarting(false);
      setInstallError(
        getErrorMessage(err) ||
          "Failed to restart automatically. Please quit and reopen Mail Pulse.",
      );
    }
  };

  const handleOpenGitHub = async () => {
    const url =
      checkResult?.releaseUrl ||
      (versionInfo?.repo
        ? `https://github.com/${versionInfo.repo}/releases`
        : "https://github.com/pankaj-kumar34/mail-pulse/releases");
    try {
      await UpdaterService.OpenReleasePage(url);
    } catch {
      window.open(url, "_blank", "noopener,noreferrer");
    }
  };

  if (!isOpen) return null;

  const currentVer =
    checkResult?.currentVersion || versionInfo?.version || "1.0.0";
  const latestVer = checkResult?.latestVersion || currentVer;
  const hasUpdate = !!checkResult?.hasUpdate;
  const isReadyToRestart = progress?.stage === "ready";
  const canSelfUpdate = !!(checkResult?.canSelfUpdate && checkResult?.assetUrl);

  const formattedDate = checkResult?.publishedAt
    ? new Date(checkResult.publishedAt).toLocaleDateString(undefined, {
        year: "numeric",
        month: "short",
        day: "numeric",
      })
    : null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-xs duration-150 animate-in select-none fade-in"
      onClick={() => {
        if (!installing && !restarting) onClose();
      }}
    >
      <div
        className="flex max-h-[85vh] w-full max-w-lg transform flex-col overflow-hidden rounded-2xl border border-zinc-200 bg-white shadow-2xl transition-all duration-150 scale-in-95"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Modal Header */}
        <div className="flex shrink-0 items-center justify-between border-b border-zinc-200/80 bg-zinc-50/80 px-6 py-4">
          <div className="flex items-center gap-3">
            <div
              className={`flex size-9 shrink-0 items-center justify-center rounded-xl shadow-2xs ${
                isReadyToRestart
                  ? "bg-emerald-100 text-emerald-600"
                  : hasUpdate
                    ? "bg-emerald-100 text-emerald-600"
                    : "bg-zinc-100 text-zinc-700"
              }`}
            >
              {isReadyToRestart ? (
                <CheckCircle2 size={18} />
              ) : hasUpdate ? (
                <Sparkles size={18} />
              ) : (
                <Download size={18} />
              )}
            </div>
            <div>
              <h3 className="text-sm leading-tight font-bold text-zinc-900">
                {isReadyToRestart
                  ? "Update Ready to Restart"
                  : hasUpdate
                    ? "New Update Available"
                    : "Software Update"}
              </h3>
              <p className="mt-0.5 font-mono text-[11px] text-zinc-500">
                {versionInfo?.repo || "pankaj-kumar34/mail-pulse"} •{" "}
                {versionInfo?.os || "darwin"}/{versionInfo?.arch || "arm64"}
              </p>
            </div>
          </div>

          <button
            onClick={onClose}
            disabled={installing || restarting}
            className="flex size-7 cursor-pointer items-center justify-center rounded-lg text-zinc-400 transition-colors hover:bg-zinc-200/60 hover:text-zinc-700 disabled:opacity-40"
            title="Close (Esc)"
          >
            <X size={15} />
          </button>
        </div>

        {/* Body */}
        <div className="flex-1 space-y-4 overflow-y-auto p-6">
          {/* Version Comparison Banner */}
          <div className="flex items-center justify-between gap-3 rounded-xl border border-zinc-200/80 bg-zinc-50 p-3.5">
            <div className="flex items-center gap-2.5">
              <div className="flex flex-col">
                <span className="text-[10px] font-semibold tracking-wider text-zinc-400 uppercase">
                  Installed
                </span>
                <span className="font-mono text-xs font-bold text-zinc-700">
                  v{currentVer}
                </span>
              </div>

              {hasUpdate && (
                <>
                  <ArrowRight size={14} className="mx-1 text-zinc-400" />
                  <div className="flex flex-col">
                    <span className="text-[10px] font-semibold tracking-wider text-emerald-600 uppercase">
                      Latest Release
                    </span>
                    <span className="font-mono text-xs font-bold text-emerald-600">
                      v{latestVer}
                    </span>
                  </div>
                </>
              )}
            </div>

            <div className="flex items-center gap-2">
              {formattedDate && (
                <span className="text-[11px] font-medium text-zinc-400">
                  {formattedDate}
                </span>
              )}
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={checking || installing || restarting}
                onClick={runCheck}
                className="h-7 cursor-pointer gap-1.5 border-zinc-200 bg-white px-2.5 text-[11px] font-semibold text-zinc-700 hover:bg-zinc-100"
              >
                <RefreshCw
                  size={12}
                  className={checking ? "animate-spin text-emerald-600" : ""}
                />
                <span>{checking ? "Checking..." : "Check Now"}</span>
              </Button>
            </div>
          </div>

          {/* Checking State */}
          {checking && !checkResult && (
            <div className="flex flex-col items-center justify-center gap-2.5 py-8 text-center">
              <RefreshCw size={22} className="animate-spin text-emerald-600" />
              <p className="text-xs font-semibold text-zinc-700">
                Checking GitHub Releases for updates...
              </p>
            </div>
          )}

          {/* Check Error State */}
          {checkError && (
            <div className="flex items-start gap-2.5 rounded-xl border border-rose-200 bg-rose-50 p-3.5 text-xs text-rose-700">
              <AlertTriangle
                size={15}
                className="mt-0.5 shrink-0 text-rose-600"
              />
              <div className="min-w-0 flex-1">
                <p className="font-semibold">Update Check Failed</p>
                <p className="mt-0.5 text-[11px] wrap-break-word text-rose-600/90">
                  {checkError}
                </p>
              </div>
            </div>
          )}

          {/* Up to Date State */}
          {!checking && !checkError && checkResult && !hasUpdate && (
            <div className="flex items-center gap-3.5 rounded-xl border border-emerald-200/80 bg-emerald-50/60 p-5">
              <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-emerald-100 text-emerald-600">
                <CheckCircle2 size={20} />
              </div>
              <div>
                <h4 className="text-xs font-bold text-emerald-950">
                  Mail Pulse is up to date
                </h4>
                <p className="mt-0.5 text-[11px] leading-relaxed text-emerald-700">
                  You are running the latest version (v{currentVer}).
                </p>
              </div>
            </div>
          )}

          {/* Update Available Details */}
          {!checking && checkResult && hasUpdate && (
            <div className="space-y-3.5">
              {/* Asset & Security Info */}
              {checkResult.assetName && (
                <div className="flex items-center justify-between rounded-lg border border-zinc-200/70 bg-zinc-100/80 px-3 py-2 text-[11px] text-zinc-600">
                  <div className="flex min-w-0 items-center gap-2">
                    <Package size={13} className="shrink-0 text-zinc-500" />
                    <span className="truncate font-mono font-medium text-zinc-800">
                      {checkResult.assetName}
                    </span>
                    {checkResult.assetSize > 0 && (
                      <span className="shrink-0 text-zinc-400">
                        ({formatBytes(checkResult.assetSize)})
                      </span>
                    )}
                  </div>
                  {checkResult.checksumUrl && (
                    <span
                      className="ml-2 flex shrink-0 items-center gap-1 font-semibold text-emerald-600"
                      title="Verified with SHA-256 checksums.txt"
                    >
                      <ShieldCheck size={12} /> SHA-256
                    </span>
                  )}
                </div>
              )}
            </div>
          )}

          {/* Live Download & Install Progress */}
          {(installing || progress) && (
            <div className="space-y-2 rounded-xl border border-zinc-200 bg-zinc-50 p-3.5">
              <div className="flex items-center justify-between gap-2 text-xs">
                <span className="flex min-w-0 flex-1 items-center gap-1.5 font-semibold text-zinc-800">
                  {installing && (
                    <RefreshCw
                      size={12}
                      className="shrink-0 animate-spin text-emerald-600"
                    />
                  )}
                  {isReadyToRestart && (
                    <CheckCircle2
                      size={13}
                      className="shrink-0 text-emerald-600"
                    />
                  )}
                  <span className="truncate">
                    {progress?.stage === "error"
                      ? "Installation failed"
                      : progress?.message || "Processing update..."}
                  </span>
                </span>
                <span className="shrink-0 font-mono text-[11px] font-bold text-zinc-600">
                  {Math.round(progress?.percent || 0)}%
                </span>
              </div>

              <div className="h-2 w-full overflow-hidden rounded-full bg-zinc-200">
                <div
                  className={`h-full rounded-full transition-all duration-200 ${
                    progress?.stage === "error"
                      ? "bg-rose-600"
                      : "bg-emerald-600"
                  }`}
                  style={{
                    width: `${Math.min(100, Math.max(4, progress?.percent || 0))}%`,
                  }}
                />
              </div>

              {progress &&
                progress.stage === "downloading" &&
                progress.totalBytes > 0 && (
                  <div className="flex justify-end font-mono text-[10px] text-zinc-400">
                    {formatBytes(progress.downloadedBytes)} /{" "}
                    {formatBytes(progress.totalBytes)}
                  </div>
                )}
            </div>
          )}

          {/* Install Error State */}
          {installError && (
            <div className="flex items-start gap-2.5 rounded-xl border border-rose-200 bg-rose-50 p-3.5 text-xs text-rose-700">
              <AlertTriangle
                size={15}
                className="mt-0.5 shrink-0 text-rose-600"
              />
              <div className="min-w-0 flex-1">
                <p className="font-semibold">Automatic Installation Failed</p>
                <p className="mt-0.5 text-[11px] wrap-break-word break-all text-rose-600/90">
                  {installError}
                </p>
                <p className="mt-1.5 text-[11px] font-medium text-rose-700">
                  You can download the installer directly from the GitHub
                  Release page below.
                </p>
              </div>
            </div>
          )}
        </div>

        {/* Modal Footer */}
        <div className="flex shrink-0 items-center justify-between gap-2.5 border-t border-zinc-200/80 bg-zinc-50 px-6 py-3.5">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={handleOpenGitHub}
            className="h-9 cursor-pointer gap-1.5 border-zinc-200 bg-white px-3.5 text-xs font-semibold text-zinc-700 hover:bg-zinc-100"
          >
            <ExternalLink size={13} />
            <span>GitHub Releases</span>
          </Button>

          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={installing || restarting}
              onClick={onClose}
              className="h-9 cursor-pointer border-zinc-200 bg-white px-4 text-xs font-semibold text-zinc-700 hover:bg-zinc-100"
            >
              {isReadyToRestart ? "Later" : "Close"}
            </Button>

            {isReadyToRestart ? (
              <Button
                type="button"
                variant="default"
                size="sm"
                disabled={restarting}
                onClick={handleRestartNow}
                className="h-9 cursor-pointer gap-1.5 bg-emerald-600 px-4 text-xs font-semibold text-white shadow-xs hover:bg-emerald-500"
              >
                <RotateCcw
                  size={13}
                  className={restarting ? "animate-spin" : ""}
                />
                <span>{restarting ? "Restarting..." : "Restart & Apply"}</span>
              </Button>
            ) : (
              hasUpdate &&
              (canSelfUpdate ? (
                <Button
                  type="button"
                  variant="default"
                  size="sm"
                  disabled={installing}
                  onClick={handleInstallUpdate}
                  className="h-9 cursor-pointer gap-1.5 bg-emerald-600 px-4 text-xs font-semibold text-white shadow-xs hover:bg-emerald-500"
                >
                  <Download size={13} />
                  <span>
                    {installing ? "Installing..." : `Update to v${latestVer}`}
                  </span>
                </Button>
              ) : (
                <Button
                  type="button"
                  variant="default"
                  size="sm"
                  onClick={handleOpenGitHub}
                  className="h-9 cursor-pointer gap-1.5 bg-emerald-600 px-4 text-xs font-semibold text-white shadow-xs hover:bg-emerald-500"
                >
                  <ExternalLink size={13} />
                  <span>Download v{latestVer}</span>
                </Button>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
