/**
 * 秒表设置组件
 * 提供秒表最大计时时长配置
 */

import { SettingItem } from "./SettingItem";
import { NumberInput } from "./NumberInput";
import { t } from "../utils/i18n";

interface StopwatchSettingsProps {
  /** 当前配置 */
  config: {
    /** 秒表最大允许秒数 */
    max_seconds: number;
  };
  /** 是否启用滑入动画 */
  isAnimated?: boolean;
  /** 配置变化回调 */
  onChange: (config: any) => void;
}

export const StopwatchSettings = ({
  config,
  onChange,
  isAnimated = true,
}: StopwatchSettingsProps) => {
  return (
    <div
      className={`space-y-4 sm:space-y-6 ${isAnimated ? "animate-slideUp" : ""}`}
      style={isAnimated ? { animationDelay: "0.3s", animationFillMode: "both" } : undefined}
    >
      <SettingItem label={t("settings.stopwatch.max_hours")}>
        <NumberInput
          value={Math.floor(config.max_seconds / 3600)}
          min={1}
          max={168}
          onChange={(value) =>
            onChange({ ...config, max_seconds: value * 3600 })
          }
          hint={t("settings.stopwatch.max_hours_hint", {
            hours: Math.floor(config.max_seconds / 3600),
          })}
        />
      </SettingItem>
    </div>
  );
};
