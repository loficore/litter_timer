import { describe, it, expect } from "vitest";
import {
  TIMER_DEFAULTS,
  STORAGE_KEYS,
  LAYOUT_DENSITY_OPTIONS,
  TIME_DISPLAY_STYLE_OPTIONS,
  TIMER_MODES,
  WALLPAPER_FALLBACK_GRADIENT,
  isAllowedWallpaperUrl,
  API_ENDPOINTS,
  APP_VERSION,
  DEFAULT_TIMEZONE,
  LANGUAGE_OPTIONS,
  THEME_MODES,
  type LayoutDensity,
  type TimeDisplayStyle,
  type Page,
} from "../../utils/constants";

describe("constants", () => {
  describe("TIMER_DEFAULTS", () => {
    it("应该定义正确的工作时长", () => {
      expect(TIMER_DEFAULTS.WORK_DURATION).toBe(25 * 60);
    });

    it("应该定义正确的休息时长", () => {
      expect(TIMER_DEFAULTS.REST_DURATION).toBe(5 * 60);
    });

    it("应该定义默认循环次数为 0", () => {
      expect(TIMER_DEFAULTS.LOOP_COUNT).toBe(0);
    });
  });

  describe("STORAGE_KEYS", () => {
    it("应该包含所有必要的存储键", () => {
      expect(STORAGE_KEYS.WALLPAPER).toBe("global_wallpaper");
      expect(STORAGE_KEYS.WALLPAPER_DEBUG).toBe("debug_wallpaper");
      expect(STORAGE_KEYS.LAYOUT_DENSITY).toBe("layout_density");
      expect(STORAGE_KEYS.TIME_DISPLAY_STYLE).toBe("time_display_style");
      expect(STORAGE_KEYS.LIGHT_STYLE).toBe("lt_light_style");
    });
  });

  describe("LAYOUT_DENSITY_OPTIONS", () => {
    it("应该包含紧凑选项", () => {
      const option = LAYOUT_DENSITY_OPTIONS.find((o) => o.value === "compact");
      expect(option).toBeDefined();
      expect(option?.label).toBe("紧凑");
    });

    it("应该包含标准选项", () => {
      const option = LAYOUT_DENSITY_OPTIONS.find((o) => o.value === "normal");
      expect(option).toBeDefined();
      expect(option?.label).toBe("标准");
    });

    it("应该包含宽松选项", () => {
      const option = LAYOUT_DENSITY_OPTIONS.find((o) => o.value === "spacious");
      expect(option).toBeDefined();
      expect(option?.label).toBe("宽松");
    });

    it("应该导出正确的类型", () => {
      const density: LayoutDensity = "compact";
      expect(density).toBe("compact");
    });
  });

  describe("TIME_DISPLAY_STYLE_OPTIONS", () => {
    it("应该包含经典风格", () => {
      const option = TIME_DISPLAY_STYLE_OPTIONS.find((o) => o.value === "classic");
      expect(option).toBeDefined();
      expect(option?.label).toBe("经典");
    });

    it("应该包含数码管风格", () => {
      const option = TIME_DISPLAY_STYLE_OPTIONS.find((o) => o.value === "seven_segment");
      expect(option).toBeDefined();
      expect(option?.label).toBe("数码管");
    });

    it("应该导出正确的类型", () => {
      const style: TimeDisplayStyle = "seven_segment";
      expect(style).toBe("seven_segment");
    });
  });

  describe("TIMER_MODES", () => {
    it("应该定义倒计时模式", () => {
      expect(TIMER_MODES.COUNTDOWN).toBe("countdown");
    });

    it("应该定义秒表模式", () => {
      expect(TIMER_MODES.STOPWATCH).toBe("stopwatch");
    });
  });

  describe("WALLPAPER_FALLBACK_GRADIENT", () => {
    it("应该是一个有效的 CSS 渐变", () => {
      expect(WALLPAPER_FALLBACK_GRADIENT).toContain("linear-gradient");
      expect(WALLPAPER_FALLBACK_GRADIENT).toContain("135deg");
    });
  });

  describe("isAllowedWallpaperUrl", () => {
    it("应该允许 local: 前缀的本地文件引用", () => {
      expect(isAllowedWallpaperUrl("local:x.png")).toBe(true);
    });

    it("应该允许以 / 开头的本地路径", () => {
      expect(isAllowedWallpaperUrl("/api/wallpapers/x.png")).toBe(true);
    });

    it("应该拒绝未知的 http 域名", () => {
      expect(isAllowedWallpaperUrl("https://evil.example.com/x.png")).toBe(false);
    });
  });

  describe("API_ENDPOINTS", () => {
    it("应该包含所有必要的 API 端点", () => {
      expect(API_ENDPOINTS.STATE).toBe("/api/state");
      expect(API_ENDPOINTS.START).toBe("/api/start");
      expect(API_ENDPOINTS.PAUSE).toBe("/api/pause");
      expect(API_ENDPOINTS.RESET).toBe("/api/reset");
      expect(API_ENDPOINTS.MODE).toBe("/api/mode");
      expect(API_ENDPOINTS.SETTINGS).toBe("/api/settings");
      expect(API_ENDPOINTS.TIMER_PROGRESS).toBe("/api/timer/progress");
      expect(API_ENDPOINTS.TIMER_FINISH).toBe("/api/timer/finish");
      expect(API_ENDPOINTS.TIMER_REST).toBe("/api/timer/rest");
      expect(API_ENDPOINTS.HABIT_SETS).toBe("/api/habit-sets");
      expect(API_ENDPOINTS.HABITS).toBe("/api/habits");
      expect(API_ENDPOINTS.SESSIONS).toBe("/api/sessions");
      expect(API_ENDPOINTS.EVENTS).toBe("/api/events");
    });
  });

  describe("APP_VERSION", () => {
    it("应该是一个有效的版本字符串", () => {
      expect(APP_VERSION).toMatch(/^\d+\.\d+\.\d+$/);
    });
  });

  describe("DEFAULT_TIMEZONE", () => {
    it("应该默认时区为 +8", () => {
      expect(DEFAULT_TIMEZONE).toBe(8);
    });
  });

  describe("LANGUAGE_OPTIONS", () => {
    it("应该包含中文选项", () => {
      const option = LANGUAGE_OPTIONS.find((o) => o.value === "ZH");
      expect(option).toBeDefined();
      expect(option?.label).toBe("中文");
    });

    it("应该包含英文选项", () => {
      const option = LANGUAGE_OPTIONS.find((o) => o.value === "EN");
      expect(option).toBeDefined();
      expect(option?.label).toBe("English");
    });
  });

  describe("THEME_MODES", () => {
    it("应该定义亮色模式", () => {
      expect(THEME_MODES.LIGHT).toBe("light");
    });

    it("应该定义暗色模式", () => {
      expect(THEME_MODES.DARK).toBe("dark");
    });

    it("应该定义自动模式", () => {
      expect(THEME_MODES.AUTO).toBe("auto");
    });
  });

  describe("Page 类型", () => {
    it("应该支持所有页面类型", () => {
      const pages: Page[] = ["timer", "habits", "stats", "settings"];
      pages.forEach((page) => {
        expect(["timer", "habits", "stats", "settings"]).toContain(page);
      });
    });
  });
});
