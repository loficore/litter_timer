/**
 * 骨架屏组件集合
 * Skeleton 单块占位、SkeletonText 多行文字占位、SkeletonCard 卡片占位
 */

import type { FunctionalComponent } from "preact";

interface SkeletonProps {
  /** 占位块宽度（CSS 长度） */
  width?: string;
  /** 占位块高度（CSS 长度） */
  height?: string;
  /** 自定义 className */
  className?: string;
}

export const Skeleton: FunctionalComponent<SkeletonProps> = ({
  width = "100%",
  height = "1rem",
  className = "",
}) => {
  return (
    <div
      className={`skeleton rounded ${className}`}
      style={{ width, height }}
      aria-hidden="true"
    />
  );
};

/**
 * 多行文字骨架屏属性
 */
interface SkeletonTextProps {
  /** 行数 */
  lines?: number;
  /** 自定义 className */
  className?: string;
}

export const SkeletonText: FunctionalComponent<SkeletonTextProps> = ({
  lines = 3,
  className = "",
}) => {
  return (
    <div className={`space-y-2 ${className}`}>
      {Array.from({ length: lines }).map((_, i) => (
        <Skeleton
          key={i}
          width={i === lines - 1 ? "70%" : "100%"}
          height="0.75rem"
        />
      ))}
    </div>
  );
};

/**
 * 卡片骨架屏属性
 */
interface SkeletonCardProps {
  /** 自定义 className */
  className?: string;
}

export const SkeletonCard: FunctionalComponent<SkeletonCardProps> = ({
  className = "",
}) => {
  return (
    <div className={`p-4 rounded-lg ${className}`}>
      <Skeleton height="1.5rem" width="60%" className="mb-3" />
      <SkeletonText lines={2} />
    </div>
  );
};