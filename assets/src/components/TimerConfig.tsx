/**
 * 计时器配置面板组件
 * 包含工作时长、休息时长、轮次设置
 */

import { memo } from "preact/compat";
import type { FunctionalComponent } from "preact";
import { PickerNumberInput } from "./PickerNumberInput";
import { t } from "../utils/i18n";

/**
 * 计时器配置数据结构
 */
export interface TimerConfigData {
  /** 当前模式 */
  mode: "stopwatch" | "countdown";
  /** 工作时长（秒） */
  workDuration: number;
  /** 休息时长（秒） */
  restDuration: number;
  /** 循环轮次（0 表示无限循环） */
  loopCount: number;
}

interface TimerConfigProps {
  /** 当前配置数据 */
  config: TimerConfigData;
  /** 计时器是否正在运行（运行时隐藏该面板） */
  isRunning: boolean;
  /** 是否为倒计时模式（仅倒计时模式显示配置） */
  isCountdownMode: boolean;
  /** 配置变更回调（支持部分更新） */
  onChange: (config: Partial<TimerConfigData>) => void;
}

export const TimerConfig: FunctionalComponent<TimerConfigProps> = memo(({
  config,
  isRunning,
  isCountdownMode,
  onChange,
}) => {
  if (!isCountdownMode || isRunning) {
    return null;
  }

  return (
    <div className="my-surface-card rounded-xl p-4 sm:p-5 mb-4 sm:mb-6 w-full max-w-[560px] self-center">
      <div className="flex gap-2 sm:gap-3 text-sm">
        <div className="flex-1">
          <label className="text-xs text-[var(--my-on-surface-variant)] block mb-1">{t("timer.work_min")}</label>
          <PickerNumberInput
            value={Math.floor(config.workDuration / 60)}
            min={1}
            max={999}
            onChange={(val: number) => onChange({ workDuration: val * 60 })}
            dataTestId="work-duration"
          />
        </div>
        <div className="flex-1">
          <label className="text-xs text-[var(--my-on-surface-variant)] block mb-1">{t("timer.rest_min")}</label>
          <PickerNumberInput
            value={Math.floor(config.restDuration / 60)}
            min={0}
            max={60}
            onChange={(val: number) => onChange({ restDuration: val * 60 })}
          />
        </div>
        <div className="flex-1">
          <label className="text-xs text-[var(--my-on-surface-variant)] block mb-1">{t("timer.rounds")}</label>
          <PickerNumberInput
            value={config.loopCount === 0 ? 0 : config.loopCount}
            min={0}
            max={99}
            onChange={(val: number) => onChange({ loopCount: val })}
            unit={config.loopCount === 0 ? t("timer.infinity") : undefined}
          />
        </div>
      </div>
    </div>
  );
});

TimerConfig.displayName = "TimerConfig";