/**
 * 标签页容器组件
 * 上方为可切换的标签栏，下方为对应内容区域
 */

import type { ComponentChildren, VNode } from "preact";

/**
 * 单个标签配置
 */
interface Tab {
  /** 标签唯一标识 */
  id: string;
  /** 标签显示文本 */
  label: string;
  /** 标签可选图标 */
  icon?: VNode | null | undefined;
}

interface TabPanelProps {
  /** 标签列表 */
  tabs: Tab[];
  /** 当前激活标签的 id */
  activeTab: string;
  /** 切换标签回调 */
  onTabChange: (tabId: string) => void;
  /** 标签页内容 */
  children: ComponentChildren;
  /** 是否启用滑入动画 */
  isAnimated?: boolean;
}

export const TabPanel = ({
  tabs,
  activeTab,
  onTabChange,
  children,
  isAnimated = false,
}: TabPanelProps) => {
  return (
    <div className="flex flex-col flex-1 overflow-hidden">
      <div className={`my-tabs ${isAnimated ? "animate-slideUp" : ""}`}>
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => onTabChange(tab.id)}
            className={`my-tab ${activeTab === tab.id ? "my-tab-active" : ""}`}
          >
            {tab.icon && <span className="w-5 h-5 flex items-center">{tab.icon}</span>}
            <span className="text-[0.72rem] sm:text-sm leading-none">{tab.label}</span>
          </button>
        ))}
      </div>
      <div
        className={`flex-1 overflow-y-auto p-4 flex flex-col gap-4 ${
          isAnimated ? "animate-slideUp" : ""
        }`}
        style={
          isAnimated
            ? { animationDelay: "0.25s", animationFillMode: "both" }
            : {}
        }
      >
        {children}
      </div>
    </div>
  );
};
