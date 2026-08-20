import { useState, useEffect, useCallback, useRef } from "preact/hooks";
import { Sidebar } from "./components/Sidebar";
import { TimerPage } from "./TimerPage";
import { HabitsPage } from "./HabitsPage";
import { SettingsPage } from "./Settings.tsx";
import { StatsPage } from "./Stats.tsx";
import { WallpaperGalleryPage } from "./WallpaperGalleryPage";
import { ErrorBoundary } from "./components/ErrorBoundary";
import { ToastContainer, showToast } from "./components/common/Toast";
import { getFrontendLogLevel, isPerfDebugEnabled, isWebViewRuntime, logError, logLifecycle, logPerf } from "./utils/logger";
import { useAppSettings, logWallpaperDebug } from "./hooks/useAppSettings";
import { resolveWallpaperUrl, WALLPAPER_FALLBACK_GRADIENT } from "./utils/constants";
import { t } from "./utils/i18n";
import { TimerIconComponent, HabitsIconComponent, ChartIcon, SettingsIcon, PhotoIconComponent } from "./utils/icons";

type Page = "timer" | "habits" | "stats" | "settings" | "gallery";

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

/**
 * 图片壁纸探测结果缓存：URL → 是否可加载。
 * 同一图片在切换页面/设置时不会被重复探测。
 */
const probeCache = new Map<string, boolean>();

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
    const html = document.documentElement;

    if (!wp) {
      html.style.background = "";
      html.style.backgroundImage = "";
      html.style.backgroundSize = "";
      html.style.backgroundPosition = "";
      html.style.backgroundAttachment = "";
      return;
    }

    // 渐变 / 纯色壁纸：直接应用（行为保持不变）
    if (wp.startsWith("linear") || wp.startsWith("#")) {
      html.style.backgroundImage = "";
      html.style.background = wp;
      try {
        localStorage.setItem("global_wallpaper", wp);
      } catch {
        // ignore
      }
      return;
    }

    // 图片壁纸：先探测可加载性，加载失败时优雅降级为回退渐变。
    const imgUrl = resolveWallpaperUrl(wp);
    let cancelled = false;

    const applyImage = () => {
      html.style.background = "";
      html.style.backgroundImage = `url(${imgUrl})`;
      html.style.backgroundSize = "cover";
      html.style.backgroundPosition = "center";
      html.style.backgroundAttachment = "fixed";
    };

    const applyFallback = () => {
      html.style.backgroundImage = "";
      html.style.background = WALLPAPER_FALLBACK_GRADIENT;
    };

    const cached = probeCache.get(imgUrl);
    if (cached !== undefined) {
      if (cached) {
        applyImage();
      } else {
        applyFallback();
      }
      try {
        localStorage.setItem("global_wallpaper", wp);
      } catch {
        // ignore
      }
      return;
    }

    const probe = new Image();
    probe.onload = () => {
      if (cancelled) return;
      probeCache.set(imgUrl, true);
      applyImage();
      try {
        localStorage.setItem("global_wallpaper", wp);
      } catch {
        // ignore
      }
    };
    probe.onerror = () => {
      if (cancelled) return;
      probeCache.set(imgUrl, false);
      applyFallback();
      try {
        localStorage.setItem("global_wallpaper", wp);
      } catch {
        // ignore
      }
    };
    probe.src = imgUrl;

    return () => {
      cancelled = true;
      probe.onload = null;
      probe.onerror = null;
      probe.src = "";
    };
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
            {page === "gallery" && <WallpaperGalleryPage />}
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
          <TimerIconComponent className="h-5 w-5" />
          <span className="btm-nav-label">{t("nav.timer")}</span>
        </button>
        <button
          type="button"
          data-testid="nav-habits"
          className={`my-bottom-nav-item ${page === "habits" ? "active" : ""}`}
          onClick={() => navigateTo("habits")}
        >
          <HabitsIconComponent className="h-5 w-5" />
          <span className="btm-nav-label">{t("nav.habits")}</span>
        </button>
        <button
          type="button"
          data-testid="nav-stats"
          className={`my-bottom-nav-item ${page === "stats" ? "active" : ""}`}
          onClick={() => navigateTo("stats")}
        >
          <ChartIcon className="h-5 w-5" />
          <span className="btm-nav-label">{t("nav.stats")}</span>
        </button>
        <button
          type="button"
          data-testid="nav-settings"
          className={`my-bottom-nav-item ${page === "settings" ? "active" : ""}`}
          onClick={() => navigateTo("settings")}
        >
          <SettingsIcon className="h-5 w-5" />
          <span className="btm-nav-label">{t("nav.settings")}</span>
        </button>
        <button
          type="button"
          data-testid="nav-gallery"
          className={`my-bottom-nav-item ${page === "gallery" ? "active" : ""}`}
          onClick={() => navigateTo("gallery")}
        >
          <PhotoIconComponent className="h-5 w-5" />
          <span className="btm-nav-label">{t("nav.gallery")}</span>
        </button>
      </nav>
    </>
  );
};
