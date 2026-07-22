/**
 * 倒计时设置组件
 * 提供倒计时默认时长、循环开关、循环次数、循环间隔等配置项
 */

import { SettingItem } from "./SettingItem";
import { TimeInput } from "./TimeInput";
import { NumberInput } from "./NumberInput";
import { CheckboxInput } from "./CheckboxInput";
import { t } from "../utils/i18n";

interface CountdownSettingsProps {
  /** 当前配置 */
  config: {
    /** 倒计时总秒数 */
    duration_seconds: number;
    /** 是否启用循环 */
    loop: boolean;
    /** 循环次数（0 表示无限） */
    loop_count: number;
    /** 循环间隔秒数 */
    loop_interval_seconds: number;
  };
  /** 配置变化回调 */
  onChange: (config: any) => void;
  /** 当非倒计时模式时隐藏循环相关配置 */
  showLoopControls?: boolean;
  /** 是否启用滑入动画 */
  isAnimated?: boolean;
}

export const CountdownSettings = ({
  config,
  onChange,
  showLoopControls = true,
  isAnimated = true,
}: CountdownSettingsProps) => {
  return (
    <div
      className={`space-y-4 sm:space-y-6 ${isAnimated ? "animate-slideUp" : ""}`}
      style={isAnimated ? { animationDelay: "0.3s", animationFillMode: "both" } : undefined}
    >
      <SettingItem label={t("settings.countdown.duration")}>
        <TimeInput
          value={config.duration_seconds}
          maxHours={24}
          onChange={(totalSeconds) =>
            onChange({ ...config, duration_seconds: totalSeconds })
          }
          hint={t("settings.countdown.duration_hint", {
            minutes: Math.floor(config.duration_seconds / 60),
          })}
        />
      </SettingItem>

      {showLoopControls && (
        <>
          <SettingItem label={t("settings.countdown.loop_mode")}>
            <CheckboxInput
              value={config.loop}
              onChange={(checked) => onChange({ ...config, loop: checked })}
              label={t("settings.countdown.loop_enable")}
            />
          </SettingItem>

          {config.loop && (
            <>
              <SettingItem label={t("settings.countdown.loop_count")}>
                <NumberInput
                  value={config.loop_count}
                  min={0}
                  max={100}
                  onChange={(value) =>
                    onChange({ ...config, loop_count: value })
                  }
                  hint={t("settings.countdown.loop_count_hint")}
                />
              </SettingItem>

              <SettingItem label={t("settings.countdown.loop_interval")}>
                <TimeInput
                  value={config.loop_interval_seconds}
                  maxHours={1}
                  showHours={false}
                  onChange={(totalSeconds) =>
                    onChange({ ...config, loop_interval_seconds: totalSeconds })
                  }
                  hint={t("settings.countdown.loop_interval_hint")}
                />
              </SettingItem>
            </>
          )}
        </>
      )}
    </div>
  );
};
