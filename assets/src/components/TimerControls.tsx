/**
 * 计时器控制按钮组件
 * 包含开始、暂停、继续、重置、跳过、结束按钮
 */

import { memo } from "preact/compat";
import type { FunctionalComponent } from "preact";
import { t } from "../utils/i18n";

interface TimerControlsProps {
  /** 计时器是否正在运行 */
  isRunning: boolean;
  /** 计时器是否已暂停 */
  isPaused: boolean;
  /** 计时器是否已结束 */
  isFinished: boolean;
  /** 是否为倒计时模式（倒计时才显示"跳过"按钮） */
  isCountdownMode: boolean;
  /** 开始计时回调 */
  onStart: () => void;
  /** 暂停回调 */
  onPause: () => void;
  /** 继续回调 */
  onResume: () => void;
  /** 重置回调 */
  onReset: () => void;
  /** 跳过当前阶段回调（仅倒计时） */
  onSkip: () => void;
  /** 结束计时回调 */
  onFinish: () => void;
}

export const TimerControls: FunctionalComponent<TimerControlsProps> = memo(({
  isRunning,
  isPaused,
  isFinished,
  isCountdownMode,
  onStart,
  onPause,
  onResume,
  onReset,
  onSkip,
  onFinish,
}) => {
  return (
    <div className="flex flex-wrap items-center justify-center gap-3 sm:gap-4 mt-2">
      {isRunning && !isPaused ? (
        <button
          className="btn btn-primary btn-lg min-w-[130px]"
          onClick={onPause}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          {t("timer.pause")}
        </button>
      ) : isPaused ? (
        <button
          className="btn btn-primary btn-lg min-w-[130px]"
          onClick={onResume}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
          </svg>
          {t("timer.resume")}
        </button>
      ) : (
        <button
          className="btn btn-primary btn-lg min-w-[130px]"
          onClick={onStart}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
          </svg>
          {isFinished ? t("timer.restart") : t("timer.start")}
        </button>
      )}
      
      {isCountdownMode && isRunning && (
        <button
          className="btn btn-ghost btn-lg min-w-[110px]"
          onClick={onSkip}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 5l7 7-7 7M5 5l7 7-7 7" />
          </svg>
          {t("timer.skip")}
        </button>
      )}

      {isRunning && (
        <button
          className="btn btn-success btn-lg min-w-[110px]"
          onClick={onFinish}
        >
          <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
          </svg>
          {t("timer.finish")}
        </button>
      )}
      
      <button
        className="btn btn-ghost btn-lg min-w-[110px]"
        onClick={onReset}
      >
        <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
        {t("timer.reset")}
      </button>
    </div>
  );
});

TimerControls.displayName = "TimerControls";