/**
 * 基础设置组件
 * 提供时区、语言、默认模式、主题、声音、调试等通用设置项
 */

import { SettingItem } from "./SettingItem";
import { SelectInput } from "./SelectInput";
import { t } from "../utils/i18n";
import { setPerfDebugEnabled } from "../utils/logger";
import { useState, useEffect } from "preact/hooks";

interface BasicSettingsProps {
  /** 当前设置配置对象 */
  config: {
    /** 时区偏移量（小时，-12 ~ +14） */
    timezone: number;
    /** 语言代码 */
    language: string;
    /** 启动时默认进入的模式 */
    default_mode: string;
    /** 主题模式 */
    theme_mode: string;
    /** 当前壁纸标识 */
    wallpaper?: string;
    /** 是否启用声音 */
    sound_enabled: boolean;
    /** 是否启用滴答声 */
    sound_tick: boolean;
    /** 是否启用结束提示音 */
    sound_finish: boolean;
    /** 声音音量（0-100） */
    sound_volume: number;
    /** 布局密度 */
    layout_density?: string;
    /** 时间显示样式 */
    time_display_style?: string;
    /** 浅色主题样式 */
    light_style?: string;
    /** 是否启用调试模式 */
    debug_mode?: boolean;
  };
  /** 是否启用滑入动画 */
  isAnimated?: boolean;
  /** 点击壁纸选择按钮的回调 */
  onWallpaperClick?: () => void;
  /** 配置变化回调 */
  onChange: (config: any) => void;
}

const DEBUG_STORAGE_KEY = "lt_debug_perf";

interface PerformanceMemoryInfo {
  usedJSHeapSize: number;
  totalJSHeapSize: number;
}

const getPerformanceMemory = (): PerformanceMemoryInfo | null => {
  const memory = (performance as Performance & { memory?: PerformanceMemoryInfo }).memory;
  return memory ?? null;
};

const saveDebugMode = (enabled: boolean) => {
  if (typeof window === "undefined") return;
  try {
    if (enabled) {
      localStorage.setItem(DEBUG_STORAGE_KEY, "1");
    } else {
      localStorage.removeItem(DEBUG_STORAGE_KEY);
    }
  } catch {
    // 忽略
  }
};

export const BasicSettings = ({ config, onChange, isAnimated = true, onWallpaperClick }: BasicSettingsProps) => {

  const timezones = Array.from({ length: 27 }, (_, i) => i - 12).map((tz) => ({
    value: tz,
    label: `UTC${tz >= 0 ? "+" : ""}${tz}`,
  }));

  const languages = [
    { value: "ZH", label: t("settings.basic.lang_zh") },
    { value: "EN", label: t("settings.basic.lang_en") },
    { value: "JP", label: t("settings.basic.lang_jp") },
  ];

  const modes = [
    { value: "countdown", label: t("settings.basic.mode_countdown") },
    { value: "stopwatch", label: t("settings.basic.mode_stopwatch") },
    { value: "world_clock", label: t("settings.basic.mode_world_clock") },
  ];

  const themeOptions = [
    { value: "auto", label: t("settings.basic.theme_auto") },
    { value: "light", label: t("settings.basic.theme_light") },
    { value: "dark", label: t("settings.basic.theme_dark") },
  ];

  const densityOptions = [
    { value: "compact", label: t("settings.basic.layout_compact") },
    { value: "normal", label: t("settings.basic.layout_normal") },
    { value: "spacious", label: t("settings.basic.layout_spacious") },
  ];

  const timeDisplayStyleOptions = [
    { value: "classic", label: t("settings.basic.time_display_classic") },
    { value: "seven_segment", label: t("settings.basic.time_display_seven_segment") },
  ];

  const lightStyleOptions = [
    { value: "paper", label: t("settings.basic.light_style_paper") },
    { value: "mist", label: t("settings.basic.light_style_mist") },
  ];

  const DebugInfoDisplay = () => {
    const [memInfo, setMemInfo] = useState<{
      used: string;
      total: string;
    } | null>(null);

    useEffect(() => {
      const updateMemory = () => {
        const memory = getPerformanceMemory();
        if (memory) {
          const usedMB = (memory.usedJSHeapSize / 1048576).toFixed(1);
          const totalMB = (memory.totalJSHeapSize / 1048576).toFixed(1);
          setMemInfo({ used: usedMB, total: totalMB });
        }
      };

      updateMemory();
      const interval = setInterval(updateMemory, 2000);
      return () => clearInterval(interval);
    }, []);

    if (!memInfo) {
      return (
        <div className="text-[var(--my-on-surface-variant)]">
          {t("settings.basic.debug_no_memory")}
        </div>
      );
    }

    return (
      <div className="text-[var(--my-on-surface-variant)] space-y-1">
        <div>JS Heap: {memInfo.used}MB / {memInfo.total}MB</div>
        <div className="text-[10px] opacity-60">
          {t("settings.basic.debug_memory_hint")}
        </div>
      </div>
    );
  };

  return (
    <div
      className={`space-y-4 sm:space-y-6`}
      style={isAnimated ? { animationDelay: "0.3s", animationFillMode: "both" } : undefined}
    >
      <div className="form-control mb-6">
        <label className="label">
          <span className="label-text">{t("modal.wallpaper")}</span>
        </label>
        <button
          type="button"
          className="w-full h-24 rounded-xl border-2 border-[var(--my-outline)] bg-[var(--my-surface-strong)] hover:border-[var(--my-primary)] transition-colors flex items-center justify-center gap-3"
          onClick={() => onWallpaperClick?.()}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 text-[var(--my-on-surface-variant)]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
          </svg>
          <span className="text-[var(--my-on-surface-variant)] font-medium">{t("modal.select_wallpaper")}</span>
        </button>
      </div>

      <SettingItem label={t("settings.basic.timezone")}>
        <SelectInput
          value={config.timezone}
          options={timezones}
          onChange={(value) =>
            onChange({ ...config, timezone: parseInt(value) })
          }
          hint={t("settings.basic.timezone_hint", {
            offset: `${config.timezone >= 0 ? "+" : ""}${config.timezone}`,
          })}
        />
      </SettingItem>

      <SettingItem label={t("settings.basic.language")}>
        <SelectInput
          value={config.language}
          options={languages}
          onChange={(value) => onChange({ ...config, language: value })}
        />
      </SettingItem>

      <SettingItem label={t("settings.basic.default_mode")}>
        <SelectInput
          value={config.default_mode}
          options={modes}
          onChange={(value) => onChange({ ...config, default_mode: value })}
        />
      </SettingItem>

      <SettingItem label={t("settings.basic.theme_mode")}>
        <SelectInput
          value={config.theme_mode || "dark"}
          options={themeOptions}
          onChange={(value) => onChange({ ...config, theme_mode: value })}
        />
      </SettingItem>

      <SettingItem label={t("settings.basic.layout_density")}>
        <SelectInput
          value={config.layout_density || "normal"}
          options={densityOptions}
          onChange={(value) => onChange({ ...config, layout_density: value })}
        />
      </SettingItem>

      <SettingItem label={t("settings.basic.time_display_style")}>
        <SelectInput
          value={config.time_display_style || "classic"}
          options={timeDisplayStyleOptions}
          onChange={(value) => onChange({ ...config, time_display_style: value })}
        />
      </SettingItem>

      <SettingItem label={t("settings.basic.light_style")}>
        <SelectInput
          value={config.light_style || "paper"}
          options={lightStyleOptions}
          onChange={(value) => onChange({ ...config, light_style: value })}
          hint={t("settings.basic.light_style_hint")}
        />
      </SettingItem>

      <SettingItem label={t("settings.basic.sound_enabled")}>
        <div className="space-y-3">
          <label className="label cursor-pointer rounded-lg px-3 py-2 bg-[var(--my-surface-strong)]/50 border border-[var(--my-outline)]/50">
            <span className="text-[var(--my-on-surface)]">{t("settings.basic.sound_finish")}</span>
            <input
              type="checkbox"
              className="toggle toggle-primary"
              checked={config.sound_finish}
              disabled={!config.sound_enabled}
              onChange={(e) =>
                onChange({
                  ...config,
                  sound_finish: e.currentTarget.checked,
                })
              }
            />
          </label>

          <label className="label cursor-pointer rounded-lg px-3 py-2 bg-[var(--my-surface-strong)]/50 border border-[var(--my-outline)]/50">
            <span className="text-[var(--my-on-surface)]">{t("settings.basic.sound_tick")}</span>
            <input
              type="checkbox"
              className="toggle toggle-secondary"
              checked={config.sound_tick}
              disabled={!config.sound_enabled}
              onChange={(e) =>
                onChange({
                  ...config,
                  sound_tick: e.currentTarget.checked,
                })
              }
            />
          </label>

          <label className="label cursor-pointer rounded-lg px-3 py-2 bg-[var(--my-surface-strong)]/50 border border-[var(--my-outline)]/50">
            <span className="text-[var(--my-on-surface)] font-medium">{t("settings.basic.sound_master_switch")}</span>
            <input
              type="checkbox"
              className="toggle toggle-accent"
              checked={config.sound_enabled}
              onChange={(e) =>
                onChange({
                  ...config,
                  sound_enabled: e.currentTarget.checked,
                })
              }
            />
          </label>

          <div className="rounded-lg px-3 py-3 bg-[var(--my-surface-strong)]/30 border border-[var(--my-outline)]/30">
            <div className="flex items-center justify-between text-sm mb-2">
              <span className="text-[var(--my-on-surface)]">{t("settings.basic.sound_volume")}</span>
              <span className="text-[var(--my-on-surface)] font-medium">{config.sound_volume}%</span>
            </div>
            <input
              type="range"
              min={0}
              max={100}
              step={1}
              value={config.sound_volume}
              disabled={!config.sound_enabled}
              onInput={(e) => {
                const target = e.currentTarget;
                onChange({ ...config, sound_volume: parseInt(target.value, 10) });
              }}
              className="range range-primary range-sm"
            />
          </div>
        </div>
      </SettingItem>

      <SettingItem label={t("settings.basic.debug_mode")}>
        <div className="space-y-2">
          <label className="label cursor-pointer rounded-lg px-3 py-2 bg-[var(--my-surface-strong)]/50 border border-[var(--my-outline)]/50">
            <span className="text-[var(--my-on-surface)]">{t("settings.basic.debug_mode_desc")}</span>
            <input
              type="checkbox"
              className="toggle toggle-primary"
              checked={config.debug_mode === true}
              onChange={(e) => {
                const enabled = e.currentTarget.checked;
                saveDebugMode(enabled);
                setPerfDebugEnabled(enabled);
                onChange({ ...config, debug_mode: enabled });
              }}
            />
          </label>
          {config.debug_mode === true && (
            <div className="rounded-lg px-3 py-2 bg-[var(--my-surface-strong)]/30 border border-[var(--my-outline)]/30 text-xs">
              <DebugInfoDisplay />
            </div>
          )}
        </div>
      </SettingItem>
    </div>
  );
};
