/**
 * 设置项组件
 * 一行标签 + 控件的标准设置项布局
 */

import type { ComponentChildren } from "preact";

interface SettingItemProps {
  /** 显示在左侧的标签 */
  label: string;
  /** 右侧控件内容 */
  children: ComponentChildren;
}

export const SettingItem = ({ label, children }: SettingItemProps) => {
  return (
    <div className="flex flex-col sm:flex-row sm:items-start gap-3 sm:gap-4 md:gap-6 mb-4 sm:mb-6">
      <label className="sm:min-w-max font-medium text-text-primary-dark sm:pt-2 text-xs sm:text-sm md:text-base flex-shrink-0">
        {label}
      </label>
      <div className="flex-1 flex flex-col gap-2 sm:gap-3">{children}</div>
    </div>
  );
};
