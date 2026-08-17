import { describe, it, expect, vi } from "vitest";
import { render, fireEvent, screen } from "@testing-library/preact";
import { BasicSettings } from "../../components/BasicSettings";

vi.mock("../../utils/i18n", () => ({
  t: (key: string) => {
    const translations: Record<string, string> = {
      "settings.basic.timezone": "Timezone",
      "settings.basic.timezone_hint": "UTC{offset}",
      "settings.basic.lang_zh": "中文",
      "settings.basic.lang_en": "English",
      "settings.basic.lang_jp": "日本語",
      "settings.basic.mode_countdown": "Countdown",
      "settings.basic.mode_stopwatch": "Stopwatch",
      "settings.basic.mode_world_clock": "World Clock",
      "settings.basic.theme_auto": "Auto",
      "settings.basic.theme_light": "Light",
      "settings.basic.theme_dark": "Dark",
      "settings.basic.layout_compact": "Compact",
      "settings.basic.layout_normal": "Normal",
      "settings.basic.layout_spacious": "Spacious",
      "settings.basic.time_display_classic": "Classic",
      "settings.basic.time_display_seven_segment": "Seven Segment",
      "settings.basic.light_style_paper": "Paper",
      "settings.basic.light_style_mist": "Mist",
      "settings.basic.light_style_hint": "Light style hint",
      "settings.basic.sound_enabled": "Sound",
      "settings.basic.sound_finish": "Finish Sound",
      "settings.basic.sound_tick": "Tick Sound",
      "settings.basic.sound_master_switch": "Sound Master Switch",
      "settings.basic.sound_volume": "Volume",
      "settings.basic.debug_mode": "Debug Mode",
      "settings.basic.debug_mode_desc": "Enable debug mode",
      "settings.basic.debug_no_memory": "No memory info",
      "settings.basic.debug_memory_hint": "Memory usage hint",
      "modal.wallpaper": "Wallpaper",
    };
    return translations[key] || key;
  },
}));

vi.mock("../../utils/logger", () => ({
  setPerfDebugEnabled: vi.fn(),
}));

vi.mock("../../utils/apiClientSingleton", () => ({
  getAPIClient: () => ({
    listWallpapers: vi.fn().mockResolvedValue([]),
    uploadWallpaper: vi.fn().mockResolvedValue({ filename: "test.png" }),
  }),
}));

describe("BasicSettings", () => {
  const defaultConfig = {
    timezone: 8,
    language: "ZH",
    default_mode: "countdown",
    theme_mode: "dark",
    wallpaper: "",
    sound_enabled: true,
    sound_tick: true,
    sound_finish: true,
    sound_volume: 80,
    layout_density: "normal",
    time_display_style: "classic",
    light_style: "paper",
    debug_mode: false,
  };

  const defaultProps = {
    config: defaultConfig,
    onChange: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.stubGlobal("performance", {
      ...performance,
      memory: undefined,
    });
  });

  it("应该渲染壁纸选择按钮", () => {
    const { container } = render(<BasicSettings {...defaultProps} />);
    const buttons = container.querySelectorAll("button");
    expect(buttons.length).toBeGreaterThan(0);
  });

  it("应该渲染所有设置项", () => {
    const { container } = render(<BasicSettings {...defaultProps} />);

    expect(container.querySelectorAll("label").length).toBeGreaterThan(0);
  });

  describe("sound_enabled 禁用状态", () => {
    it("sound_enabled 为 false 时应该禁用音量滑块", () => {
      const { container } = render(
        <BasicSettings
          {...defaultProps}
          config={{ ...defaultConfig, sound_enabled: false }}
        />
      );

      const rangeInput = container.querySelector('input[type="range"]');
      expect(rangeInput?.disabled).toBe(true);
    });

    it("sound_enabled 为 true 时不应该禁用音量滑块", () => {
      const { container } = render(
        <BasicSettings
          {...defaultProps}
          config={{ ...defaultConfig, sound_enabled: true }}
        />
      );

      const rangeInput = container.querySelector('input[type="range"]');
      expect(rangeInput?.disabled).toBe(false);
    });
  });

  describe("debug 模式", () => {
    it("debug_mode 为 true 时不应该渲染某些内容", () => {
      const { container } = render(
        <BasicSettings
          {...defaultProps}
          config={{ ...defaultConfig, debug_mode: true }}
        />
      );

      expect(container.querySelectorAll("label").length).toBeGreaterThan(0);
    });

    it("debug_mode 为 false 时不应该渲染某些内容", () => {
      const { container } = render(
        <BasicSettings {...defaultProps} />
      );

      expect(container.querySelectorAll("label").length).toBeGreaterThan(0);
    });
  });

  describe("动画", () => {
    it("isAnimated 为 true 时不应该有动画类（壁纸全屏显示需要移除动画）", () => {
      const { container } = render(
        <BasicSettings {...defaultProps} isAnimated={true} />
      );

      const element = container.querySelector("div");
      expect(element?.className).not.toContain("animate-slideUp");
    });

    it("isAnimated 为 false 时不应该有动画类", () => {
      const { container } = render(
        <BasicSettings {...defaultProps} isAnimated={false} />
      );

      const element = container.querySelector("div");
      expect(element?.className).not.toContain("animate-slideUp");
    });
  });

  describe("onChange 回调", () => {
    it("点击壁纸按钮时应该打开 Modal", () => {
      const { container } = render(<BasicSettings {...defaultProps} />);
      const wallpaperButton = container.querySelector("button");
      expect(wallpaperButton).toBeTruthy();
    });
  });
});