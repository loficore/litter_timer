/**
 * 时间显示组件
 * 居中显示当前时间字符串，运行时附带发光特效
 */

import type { FunctionalComponent } from "preact";
import { memo } from "preact/compat";
import { useMemo } from "preact/hooks";

interface TimeDisplayProps {
  /** 要显示的时间字符串（如 "12:34" 或 "1:23:45"） */
  time: string;
  /** 计时器是否正在运行，用于决定样式与发光效果 */
  isRunning: boolean;
}

export const TimeDisplay: FunctionalComponent<TimeDisplayProps> = memo(({ time, isRunning }) => {
  const className = useMemo(() => {
    const base = "text-4xl sm:text-6xl md:text-8xl font-light tracking-wider font-mono my-4 sm:my-6 md:my-6 text-center break-all time-transition";
    const colorClass = isRunning ? "text-primary" : "text-base-content";
    const glowClass = isRunning ? "time-running-glow" : "";
    return `${base} ${colorClass} ${glowClass}`.trim();
  }, [isRunning]);

  return (
    <div className={className}>
      <span className="time-value-swap">
        {time}
      </span>
    </div>
  );
});