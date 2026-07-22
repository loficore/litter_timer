/**
 * 应用共享设置 Hook
 * 统一管理跨组件的主题、壁纸、布局等设置
 */

import { useState, useEffect, useCallback, useRef } from "preact/hooks";
import { getAPIClient } from "../utils/apiClientSingleton";
import { STORAGE_KEYS } from "../utils/constants";
import { logError } from "../utils/logger";

/**
 * 应用中跨组件共享的设置。
 */
export interface AppSettings {
  /** 主题模式。 */
  theme_mode: string;
  /** 壁纸内容或地址。 */
  wallpaper: string;
  /** 布局密度。 */
  layout_density: string;
  /** 时间显示样式。 */
  time_display_style: string;
  /** 浅色主题样式。 */
  light_style: string;
}

/**
 * useAppSettings 返回的共享设置状态与操作方法。
 */
export interface UseAppSettingsReturn {
  /** 当前应用共享设置。 */
  settings: AppSettings;
  /** 是否正在加载设置。 */
  isLoading: boolean;
  /**
   * 应用主题模式。
   *
   * @param themeMode - 主题模式，未传入时使用深色主题。
   */
  applyTheme: (themeMode?: string) => void;
  /**
   * 应用浅色主题样式。
   *
   * @param lightStyle - 浅色主题样式，未传入时使用纸张样式。
   */
  applyLightStyle: (lightStyle?: string) => void;
  /**
   * 更新共享设置，未提供的字段保持不变。
   *
   * @param settings - 要更新的部分设置。
   */
  updateSettings: (settings: Partial<AppSettings>) => void;
  /**
   * 规范化壁纸值。
   *
   * @param value - 待处理的壁纸值。
   * @returns 去除首尾空白后的字符串，非字符串值返回空字符串。
   */
  normalizeWallpaper: (value: unknown) => string;
  /**
   * 校验壁纸地址是否为允许的格式。
   *
   * @param url - 待校验的壁纸地址。
   * @returns 允许的地址，格式不符合要求时返回空字符串。
   */
  sanitizeWallpaperUrl: (url: string) => string;
  /**
   * 获取当前壁纸的样式信息。
   *
   * @returns 壁纸类型及值，没有壁纸时返回 null。
   */
  getWallpaperStyle: () => { type: "gradient" | "color" | "image"; value: string } | null;
}

const DEFAULT_APP_SETTINGS: AppSettings = {
  theme_mode: "dark",
  wallpaper: "",
  layout_density: "normal",
  time_display_style: "classic",
  light_style: "paper",
};

const THEME_MODE_STORAGE_KEY = "lt_theme_mode";
const LIGHT_STYLE_STORAGE_KEY = "lt_light_style";
const WALLPAPER_STORAGE_KEY = STORAGE_KEYS.WALLPAPER;
const WALLPAPER_DEBUG_STORAGE_KEY = STORAGE_KEYS.WALLPAPER_DEBUG;

export const applyTheme = (themeMode = "dark") => {
  const html = document.documentElement;
  const theme =
    themeMode === "auto"
      ? window.matchMedia("(prefers-color-scheme: light)").matches
        ? "light"
        : "dark"
      : themeMode;

  if (theme === "light") {
    html.classList.add("light-mode");
    document.body.classList.add("light-mode");
  } else {
    html.classList.remove("light-mode");
    document.body.classList.remove("light-mode");
  }
};

export const applyLightStyle = (lightStyle = "paper") => {
  const html = document.documentElement;
  html.classList.remove("light-style-mist");
  if (lightStyle === "mist") {
    html.classList.add("light-style-mist");
  }
};

export const isWallpaperDebugEnabled = (): boolean => {
  try {
    if (typeof window === "undefined") return false;

    const search = new URLSearchParams(window.location.search);
    if (search.has("debugWallpaper")) return true;

    return localStorage.getItem(WALLPAPER_DEBUG_STORAGE_KEY) === "1";
  } catch {
    return false;
  }
};

export const logWallpaperDebug = (event: string, payload?: Record<string, unknown>) => {
  if (!isWallpaperDebugEnabled()) return;

  const time = new Date().toISOString();
  console.info("[wallpaper-debug]", time, event, payload || {});
};

const readCachedWallpaper = (): string => {
  try {
    return normalizeWallpaper(localStorage.getItem(WALLPAPER_STORAGE_KEY));
  } catch {
    return "";
  }
};

export const normalizeWallpaper = (value: unknown): string => {
  return typeof value === "string" ? value.trim() : "";
};

export const sanitizeWallpaperUrl = (url: string): string => {
  if (!url || typeof url !== "string") return "";
  const trimmed = url.trim();
  if (!trimmed) return "";
  const lower = trimmed.toLowerCase();
  if (lower.startsWith("http://") || lower.startsWith("https://") || lower.startsWith("data:image/") || lower.startsWith("/api/")) {
    return trimmed;
  }
  return "";
};

/**
 * 管理跨组件共享的主题、壁纸和布局设置。
 *
 * @returns 共享设置状态及主题、壁纸相关操作方法。
 */
export const useAppSettings = (): UseAppSettingsReturn => {
  const apiClientRef = useRef(getAPIClient());
  const [settings, setSettings] = useState<AppSettings>(DEFAULT_APP_SETTINGS);
  const [isLoading, setIsLoading] = useState(true);

  const loadSettings = useCallback(async () => {
    try {
      const cachedThemeMode = localStorage.getItem(THEME_MODE_STORAGE_KEY);
      if (cachedThemeMode) {
        applyTheme(cachedThemeMode);
      }

      const client = apiClientRef.current;
      const serverSettings = await client.getSettings();
      const basic = (serverSettings?.basic ?? {}) as unknown as Record<string, unknown>;

      const serverThemeMode = typeof basic.theme_mode === "string" ? basic.theme_mode : "dark";
      applyTheme(serverThemeMode);

      try {
        localStorage.setItem(THEME_MODE_STORAGE_KEY, serverThemeMode);
      } catch {
        // 忽略 localStorage 不可用场景
      }

      const localLayoutDensity = localStorage.getItem(STORAGE_KEYS.LAYOUT_DENSITY) || "normal";
      const localTimeDisplayStyle = localStorage.getItem(STORAGE_KEYS.TIME_DISPLAY_STYLE) || "classic";
      const localLightStyle = localStorage.getItem(LIGHT_STYLE_STORAGE_KEY) || "paper";

      const cachedWallpaper = readCachedWallpaper();
      const serverWallpaper = normalizeWallpaper(basic.wallpaper);

      logWallpaperDebug("serverSettingsLoaded", {
        serverWallpaper,
        cachedWallpaper,
      });

      setSettings({
        theme_mode: serverThemeMode,
        wallpaper: cachedWallpaper || serverWallpaper || "",
        layout_density: localLayoutDensity,
        time_display_style: localTimeDisplayStyle,
        light_style: localLightStyle,
      });

      applyLightStyle(localLightStyle);
    } catch (e) {
      logError(`加载应用设置失败: ${e}`);
      applyTheme(localStorage.getItem(THEME_MODE_STORAGE_KEY) || "dark");
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadSettings();
  }, [loadSettings]);

  useEffect(() => {
    if (settings.theme_mode) {
      applyTheme(settings.theme_mode);
      try {
        localStorage.setItem(THEME_MODE_STORAGE_KEY, settings.theme_mode);
      } catch {
        // 忽略
      }
    }
  }, [settings.theme_mode]);

  useEffect(() => {
    applyLightStyle(settings.light_style);
  }, [settings.light_style]);

  const updateSettings = useCallback((newSettings: Partial<AppSettings>) => {
    setSettings((prev) => ({ ...prev, ...newSettings }));
  }, []);

  const getWallpaperStyle = useCallback(() => {
    const wp = normalizeWallpaper(settings.wallpaper);
    if (!wp) return null;

    if (wp.startsWith("linear")) {
      return { type: "gradient" as const, value: wp };
    }

    if (wp.startsWith("#")) {
      return { type: "color" as const, value: wp };
    }

    return { type: "image" as const, value: wp };
  }, [settings.wallpaper]);

  return {
    settings,
    isLoading,
    applyTheme,
    applyLightStyle,
    updateSettings,
    normalizeWallpaper,
    sanitizeWallpaperUrl,
    getWallpaperStyle,
  };
};

export const saveAppSettings = (settings: Partial<AppSettings>) => {
  try {
    if (settings.layout_density) {
      localStorage.setItem(STORAGE_KEYS.LAYOUT_DENSITY, settings.layout_density);
    }
    if (settings.time_display_style) {
      localStorage.setItem(STORAGE_KEYS.TIME_DISPLAY_STYLE, settings.time_display_style);
    }
    if (settings.light_style) {
      localStorage.setItem(LIGHT_STYLE_STORAGE_KEY, settings.light_style);
    }
    if (settings.wallpaper !== undefined) {
      if (settings.wallpaper) {
        localStorage.setItem(WALLPAPER_STORAGE_KEY, settings.wallpaper);
      } else {
        localStorage.removeItem(WALLPAPER_STORAGE_KEY);
      }
    }
  } catch {
    // 忽略 localStorage 不可用场景
  }
};

export const dispatchSettingChange = (key: string, value: string) => {
  window.dispatchEvent(new CustomEvent("setting-change", {
    detail: { key, value }
  }));
};
