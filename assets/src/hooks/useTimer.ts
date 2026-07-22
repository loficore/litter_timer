/**
 * 计时器状态管理 Hook
 * 统一管理计时器状态和操作
 */

import { useState, useEffect, useRef, useCallback } from "preact/hooks";
import { getAPIClient } from "../utils/apiClientSingleton";
import { logSuccess, logError } from "../utils/logger";
import { formatDuration } from "../utils/formatters";

export type TimerMode = "stopwatch" | "countdown";

/**
 * 计时器配置。
 */
export interface TimerConfig {
  /** 计时模式：正计时或倒计时。 */
  mode: TimerMode;
  /** 工作阶段的时长，单位为秒。 */
  workDuration: number;
  /** 休息阶段的时长，单位为秒。 */
  restDuration: number;
  /** 循环次数，0 表示不限制次数。 */
  loopCount: number;
}

/**
 * 计时器的运行时状态。
 */
export interface TimerState {
  /** 是否正在计时。 */
  isRunning: boolean;
  /** 是否已暂停计时。 */
  isPaused: boolean;
  /** 倒计时是否已完成。 */
  isFinished: boolean;
  /** 当前是否处于休息阶段。 */
  isResting: boolean;
  /** 当前循环轮次。 */
  currentRound: number;
  /** 已计时秒数。 */
  elapsedSeconds: number;
  /** 剩余秒数。 */
  remainingSeconds: number;
}

/**
 * useTimer 返回的计时器状态与控制方法。
 */
export interface UseTimerReturn {
  /** 当前计时器配置。 */
  timerConfig: TimerConfig;
  /** 是否正在计时。 */
  isRunning: boolean;
  /** 是否已暂停计时。 */
  isPaused: boolean;
  /** 倒计时是否已完成。 */
  isFinished: boolean;
  /** 当前是否处于休息阶段。 */
  isResting: boolean;
  /** 当前循环轮次。 */
  currentRound: number;
  /** 已计时秒数。 */
  elapsedSeconds: number;
  /** 剩余秒数。 */
  remainingSeconds: number;
  /** 按当前计时模式格式化后的显示时间。 */
  displayTime: string;

  /** 更新计时器配置，未提供的字段保持不变。 */
  setTimerConfig: (config: Partial<TimerConfig>) => void;
  /** 开始计时，可选关联一个习惯 ID。 */
  start: (habitId?: number) => Promise<void>;
  /** 暂停当前计时。 */
  pause: () => Promise<void>;
  /** 恢复计时，可选关联一个习惯 ID。 */
  resume: (habitId?: number) => Promise<void>;
  /** 重置计时器状态。 */
  reset: () => Promise<void>;
  /** 跳转到倒计时的下一个工作或休息阶段。 */
  skipToNext: () => void;
  /** 完成当前计时并返回本次已计时的秒数。 */
  finish: () => Promise<{ elapsed_seconds: number }>;
}

/**
 * 管理计时器配置、运行状态和计时控制操作。
 *
 * @returns 计时器状态、格式化显示时间及控制方法。
 */
export const useTimer = (): UseTimerReturn => {
  const apiClientRef = useRef(getAPIClient());
  const rafRef = useRef<number | null>(null);
  const startTimeRef = useRef<number>(0);
  const totalElapsedRef = useRef<number>(0);
  const sessionRecordedRef = useRef(false);

  const [timerConfig, setTimerConfigState] = useState<TimerConfig>({
    mode: "stopwatch",
    workDuration: 25 * 60,
    restDuration: 5 * 60,
    loopCount: 0,
  });

  const [timerState, setTimerState] = useState<TimerState>({
    isRunning: false,
    isPaused: false,
    isFinished: false,
    isResting: false,
    currentRound: 0,
    elapsedSeconds: 0,
    remainingSeconds: 0,
  });

  const displayTime = timerConfig.mode === "stopwatch" ? timerState.elapsedSeconds : timerState.remainingSeconds;
  const displayTimeStr = formatDuration(displayTime);

  const setTimerConfig = useCallback((config: Partial<TimerConfig>) => {
    setTimerConfigState((prev) => ({ ...prev, ...config }));
  }, []);

  const start = useCallback(async (habitId?: number) => {
    if (timerState.isFinished) {
      await reset();
      return;
    }

    sessionRecordedRef.current = false;
    startTimeRef.current = Date.now();

    setTimerState((prev) => ({
      ...prev,
      isRunning: true,
      isPaused: false,
      isResting: false,
      ...(timerConfig.mode === "countdown"
        ? { remainingSeconds: timerConfig.workDuration, currentRound: 1, elapsedSeconds: 0 }
        : { elapsedSeconds: 0 }),
    }));

    try {
      await apiClientRef.current.startTimer(habitId, {
        mode: timerConfig.mode,
        workDuration: timerConfig.workDuration,
        restDuration: timerConfig.restDuration,
        loopCount: timerConfig.loopCount,
      });
    } catch (e) {
      logError(`启动计时失败: ${e}`);
    }
  }, [timerConfig, timerState.isFinished]);

  const pause = useCallback(async () => {
    setTimerState((prev) => ({ ...prev, isPaused: true }));
    try {
      await apiClientRef.current.pauseTimer();
    } catch (e) {
      logError(`暂停计时失败: ${e}`);
      throw e;
    }
  }, []);

  const resume = useCallback(async (habitId?: number) => {
    if (rafRef.current) {
      cancelAnimationFrame(rafRef.current);
      rafRef.current = null;
    }
    startTimeRef.current = Date.now() - totalElapsedRef.current * 1000;
    setTimerState((prev) => ({ ...prev, isPaused: false }));
    try {
      await apiClientRef.current.resumeTimer(habitId);
    } catch (e) {
      logError(`恢复计时失败: ${e}`);
      throw e;
    }
  }, []);

  const reset = useCallback(async () => {
    setTimerState({
      isRunning: false,
      isPaused: false,
      isFinished: false,
      isResting: false,
      currentRound: 0,
      elapsedSeconds: 0,
      remainingSeconds: timerConfig.workDuration,
    });
    sessionRecordedRef.current = false;
    startTimeRef.current = 0;
    totalElapsedRef.current = 0;

    try {
      await apiClientRef.current.resetTimer();
    } catch (e) {
      logError(`重置计时失败: ${e}`);
      throw e;
    }
  }, [timerConfig.workDuration]);

  const finish = useCallback(async (): Promise<{ elapsed_seconds: number }> => {
    try {
      const result = await apiClientRef.current.finishTimer();
      logSuccess(`✓ 已计入今日统计: ${formatDuration(result.elapsed_seconds)}`);

      setTimerState({
        isRunning: false,
        isPaused: false,
        isFinished: false,
        currentRound: 0,
        elapsedSeconds: 0,
        remainingSeconds: timerConfig.workDuration,
        isResting: false,
      });
      sessionRecordedRef.current = false;

      return result;
    } catch (e) {
      logError(`结束计时失败: ${e}`);
      throw e;
    }
  }, [timerConfig.workDuration]);

  const skipToNext = useCallback(() => {
    if (timerConfig.mode === "countdown" && timerState.isRunning) {
      if (timerState.isResting) {
        setTimerState((prev) => ({
          ...prev,
          isResting: false,
          remainingSeconds: timerConfig.workDuration,
          currentRound: prev.currentRound + 1,
        }));
      } else {
        if (timerConfig.loopCount > 0 && timerState.currentRound >= timerConfig.loopCount) {
          setTimerState((prev) => ({ ...prev, isFinished: true, isRunning: false }));
        } else if (timerConfig.restDuration > 0) {
          setTimerState((prev) => ({
            ...prev,
            isResting: true,
            remainingSeconds: timerConfig.restDuration,
          }));
        } else {
          setTimerState((prev) => ({
            ...prev,
            currentRound: prev.currentRound + 1,
            remainingSeconds: timerConfig.workDuration,
          }));
        }
      }
    }
  }, [timerConfig, timerState.isRunning, timerState.isResting, timerState.currentRound]);

  useEffect(() => {
    if (!timerState.isRunning || timerState.isPaused || timerState.isFinished) {
      if (rafRef.current) {
        cancelAnimationFrame(rafRef.current);
        rafRef.current = null;
      }
      return;
    }

    let cancelled = false;

    const tick = () => {
      if (cancelled) return;

      const wallElapsed = Math.floor((Date.now() - startTimeRef.current) / 1000);

      if (timerConfig.mode === "stopwatch") {
        totalElapsedRef.current = wallElapsed;
        setTimerState((prev) => ({ ...prev, elapsedSeconds: wallElapsed }));
      } else {
        if (timerState.isResting) {
          const restElapsed = wallElapsed - timerConfig.workDuration;
          const newVal = timerConfig.restDuration - restElapsed;
          if (newVal <= 0) {
            setTimerState((prev) => ({
              ...prev,
              isResting: false,
              remainingSeconds: timerConfig.workDuration,
              currentRound: prev.currentRound + 1,
            }));
          } else {
            setTimerState((prev) => ({ ...prev, remainingSeconds: newVal }));
          }
        } else {
          const newVal = timerConfig.workDuration - wallElapsed;
          if (newVal <= 0) {
            if (timerConfig.loopCount > 0 && timerState.currentRound >= timerConfig.loopCount) {
              setTimerState((prev) => ({ ...prev, isFinished: true, isRunning: false }));
              return;
            } else if (timerConfig.restDuration > 0) {
              setTimerState((prev) => ({
                ...prev,
                isResting: true,
                remainingSeconds: timerConfig.restDuration,
              }));
            } else {
              setTimerState((prev) => ({
                ...prev,
                currentRound: prev.currentRound + 1,
                remainingSeconds: timerConfig.workDuration,
              }));
            }
          } else {
            setTimerState((prev) => ({ ...prev, remainingSeconds: newVal }));
          }
        }
      }

      if (!cancelled) {
        rafRef.current = requestAnimationFrame(tick);
      }
    };

    rafRef.current = requestAnimationFrame(tick);

    return () => {
      cancelled = true;
      if (rafRef.current) {
        cancelAnimationFrame(rafRef.current);
        rafRef.current = null;
      }
    };
  }, [timerState.isRunning, timerState.isPaused, timerState.isFinished, timerConfig, timerState.isResting, timerState.currentRound]);

  return {
    timerConfig,
    isRunning: timerState.isRunning,
    isPaused: timerState.isPaused,
    isFinished: timerState.isFinished,
    isResting: timerState.isResting,
    currentRound: timerState.currentRound,
    elapsedSeconds: timerState.elapsedSeconds,
    remainingSeconds: timerState.remainingSeconds,
    displayTime: displayTimeStr,
    setTimerConfig,
    start,
    pause,
    resume,
    reset,
    skipToNext,
    finish,
  };
};
