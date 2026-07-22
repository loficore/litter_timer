/**
 * 状态徽章组件 - Material You 风格
 * 根据计时器状态显示对应颜色与"运行中"小圆点
 */

import type { FunctionalComponent } from "preact";

/** 计时器状态类型 */
type StatusType = "running" | "paused" | "finished";

interface StatusBadgeProps {
  /** 状态类型 */
  status: StatusType;
  /** 状态标签文本 */
  label: string;
  /** 动画延迟（可选） */
  animationDelay?: string;
}

/**
 * 获取 Material You 徽章类名
 */
const getStatusVariant = (status: StatusType): string => {
  const variants: Record<StatusType, string> = {
    running: "my-badge-running",
    paused: "my-badge-paused",
    finished: "my-badge-finished",
  };
  return variants[status];
};

/**
 * 状态徽章组件 - Material You 风格
 */
export const StatusBadge: FunctionalComponent<StatusBadgeProps> = ({
  status,
  label,
  animationDelay = "0s",
}) => {
  const variant = getStatusVariant(status);

  return (
    <span
      className={`${variant} gap-2 text-xs sm:text-sm animate-slideUp`}
      style={{ animationDelay, animationFillMode: "both" }}
    >
      {status === "running" && (
        <span className="w-2 h-2 rounded-full bg-white/80 animate-pulse" />
      )}
      {label}
    </span>
  );
};
