import { useState, useEffect, useCallback, useRef } from "preact/hooks";
import { Sidebar } from "./components/Sidebar";
import { TimerPage } from "./TimerPage";
import { HabitsPage } from "./HabitsPage";
import { SettingsPage } from "./Settings.tsx";
import { StatsPage } from "./Stats.tsx";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { ToastContainer, showToast } from "./components/common/Toast";
import { getFrontendLogLevel, isPerfDebugEnabled, isWebViewRuntime, logError, logLifecycle, logPerf } from "./utils/logger";
import { useAppSettings, logWallpaperDebug } from "./hooks/useAppSettings";
import { resolveWallpaperUrl } from "./utils/constants";
import { t } from "./utils/i18n";

type Page = "timer" | "habits" | "stats" | "settings";

const formatUnknownError = (value: unknown): string => {
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean" || typeof value === "bigint") return `${value}`;
  if (value instanceof Error) return value.message;
  if (value && typeof value === "object") {
    try {
      return JSON.stringify(value);
    } catch {
      return "[无法序列化的错误对象]";
    }
  }
  return "未知错误";
};

export const App = () => {
  const [page, setPage] = useState<Page>("timer");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const { settings, normalizeWallpaper, updateSettings } = useAppSettings();
  const previousErrorToastRef = useRef<string | null>(null);

  const navigateTo = (newPage: Page) => {
    setPage(newPage);
  };

  const globalWallpaper = settings.wallpaper;

  // 全局错误捕获
  useEffect(() => {
    if (typeof window === "undefined") return;

    logLifecycle("应用初始化");
    logLifecycle(`日志配置: level=${getFrontendLogLevel()} perf=${isPerfDebugEnabled() ? "on" : "off"}`);
    logPerf("App.perfDebug.status", {
      enabled: isPerfDebugEnabled(),
      hint: "URL 参数 debugPerf=1&logLevel=debug 或 localStorage 键 lt_debug_perf=1, lt_log_level=debug",
    });

    window.onerror = (message, _source, _lineno, _colno, error) => {
      const text = formatUnknownError(message);
      logError(`全局错误: ${text}`, error);
      setErrorMessage(text);
      return false;
    };

    window.onunhandledrejection = (event) => {
      const reasonText = formatUnknownError(event.reason);
      const reasonError = event.reason instanceof Error ? event.reason : undefined;
      logError(`未处理的 Promise 拒绝: ${reasonText}`, reasonError);
      setErrorMessage(reasonText);
    };

    logLifecycle("WebView 已渲染完成");
  }, []);

  useEffect(() => {
    if (typeof document === "undefined") return;

    const html = document.documentElement;
    const isStatsWebViewLite = isWebViewRuntime() && page === "stats";

    html.classList.toggle("webview-stats-lite", isStatsWebViewLite);

    return () => {
      html.classList.remove("webview-stats-lite");
    };
  }, [page]);

  useEffect(() => {
    const wp = settings.wallpaper;
    if (!wp) {
      document.documentElement.style.background = "";
      document.documentElement.style.backgroundImage = "";
      document.documentElement.style.backgroundSize = "";
      document.documentElement.style.backgroundPosition = "";
      document.documentElement.style.backgroundAttachment = "";
      return;
    }

    if (wp.startsWith("linear")) {
      document.documentElement.style.backgroundImage = "";
      document.documentElement.style.background = wp;
    } else if (wp.startsWith("#")) {
      document.documentElement.style.backgroundImage = "";
      document.documentElement.style.background = wp;
    } else {
      const imgUrl = resolveWallpaperUrl(wp);
      document.documentElement.style.background = "";
      document.documentElement.style.backgroundImage = `url(${imgUrl})`;
      document.documentElement.style.backgroundSize = "cover";
      document.documentElement.style.backgroundPosition = "center";
      document.documentElement.style.backgroundAttachment = "fixed";
    }

    try {
      localStorage.setItem("global_wallpaper", wp);
    } catch {
      // ignore
    }
  }, [settings.wallpaper]);

  const handleWallpaperChange = useCallback((wallpaper: string) => {
    const normalized = normalizeWallpaper(wallpaper);
    logWallpaperDebug("updateGlobalWallpaper", {
      source: "settings-prop",
      incoming: wallpaper,
      normalized,
    });
    updateSettings({ wallpaper: normalized });
  }, [normalizeWallpaper, updateSettings]);

  const handlePageError = useCallback((error: Error) => {
    setErrorMessage(error.message || "未知错误");
  }, []);

  useEffect(() => {
    if (!errorMessage) {
      previousErrorToastRef.current = null;
      return;
    }

    if (previousErrorToastRef.current === errorMessage) {
      return;
    }

    previousErrorToastRef.current = errorMessage;
    showToast(errorMessage, "error");
  }, [errorMessage]);

  return (
    <>
      <div className="flex h-screen bg-transparent">
        {/* 侧边栏 - 桌面端 */}
        <div className="hidden lg:block lg:flex shrink-0">
          <Sidebar currentPage={page} onNavigate={navigateTo} />
        </div>

        {/* 主内容区 */}
        <main className="flex-1 flex flex-col overflow-hidden pb-20 lg:pb-0">
          <ErrorBoundary onError={handlePageError}>
            <div className={page === "timer" ? "flex-1" : "hidden"}>
              <TimerPage
                onHabitsClick={() => navigateTo("habits")}
              />
            </div>
            {page === "habits" && (
              <HabitsPage
                onStatsClick={() => navigateTo("stats")}
                onSettingsClick={() => navigateTo("settings")}
              />
            )}
            {page === "stats" && (
              <StatsPage onBackClick={() => navigateTo("timer")} />
            )}
            {page === "settings" && (
              <SettingsPage
                onBackClick={() => navigateTo("timer")}
                wallpaper={globalWallpaper}
                onWallpaperChange={handleWallpaperChange}
              />
            )}
          </ErrorBoundary>
        </main>
      </div>

      <ToastContainer />

      {/* 底部导航 - 移动端 */}
      <nav
        className="my-bottom-nav lg:hidden fixed inset-x-0 bottom-0 w-full z-50"
        data-testid="bottom-nav"
      >
        <button
          type="button"
          data-testid="nav-timer"
          className={`my-bottom-nav-item ${page === "timer" ? "active" : ""}`}
          onClick={() => navigateTo("timer")}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span className="btm-nav-label">{t("nav.timer")}</span>
        </button>
        <button
          type="button"
          data-testid="nav-habits"
          className={`my-bottom-nav-item ${page === "habits" ? "active" : ""}`}
          onClick={() => navigateTo("habits")}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
          </svg>
          <span className="btm-nav-label">{t("nav.habits")}</span>
        </button>
        <button
          type="button"
          data-testid="nav-stats"
          className={`my-bottom-nav-item ${page === "stats" ? "active" : ""}`}
          onClick={() => navigateTo("stats")}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
          </svg>
          <span className="btm-nav-label">{t("nav.stats")}</span>
        </button>
        <button
          type="button"
          data-testid="nav-settings"
          className={`my-bottom-nav-item ${page === "settings" ? "active" : ""}`}
          onClick={() => navigateTo("settings")}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
          <span className="btm-nav-label">{t("nav.settings")}</span>
        </button>
      </nav>
    </>
  );
};
