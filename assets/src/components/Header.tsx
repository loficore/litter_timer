/**
 * 顶部导航栏组件
 * 提供标题、设置/统计/返回按钮的统一入口
 */

import type { FunctionalComponent } from "preact";
import { t } from "../utils/i18n";
import { ArrowLeftIconComponent, SettingsIcon, ChartIcon } from "../utils/icons";

interface HeaderProps {
  /** 标题文本 */
  title?: string;
  /** 是否显示设置按钮 */
  showSettings?: boolean;
  /** 设置按钮点击回调 */
  onSettingsClick?: () => void;
  /** 是否显示统计按钮 */
  showStats?: boolean;
  /** 统计按钮点击回调 */
  onStatsClick?: () => void;
  /** 是否显示返回按钮 */
  showBack?: boolean;
  /** 返回按钮点击回调 */
  onBackClick?: () => void;
}

export const Header: FunctionalComponent<HeaderProps> = ({
  title = t("common.app_name"),
  showSettings = true,
  onSettingsClick,
  showStats = false,
  onStatsClick,
  showBack = false,
  onBackClick,
}) => {
  return (
    <div className="navbar my-topbar shrink-0 px-2 sm:px-3">
      <div className="flex-1">
        {showBack && (
          <button
            onClick={onBackClick}
            title={t("common.back")}
            aria-label={t("common.back")}
            className="my-back-btn"
          >
            <ArrowLeftIconComponent />
            <span>{t("common.back")}</span>
          </button>
        )}
      </div>
      <div className="flex-none">
        <h1 className="my-topbar-title text-lg font-semibold">{title}</h1>
      </div>
      <div className="flex-1 flex justify-end gap-2">
        {showStats && (
          <button
            onClick={onStatsClick}
            title={t("stats.title") || "统计"}
            aria-label={t("stats.title") || "Statistics"}
            className="my-icon-btn"
          >
            <ChartIcon />
          </button>
        )}
        {showSettings && (
          <button
            onClick={onSettingsClick}
            title={t("common.settings_title")}
            aria-label={t("common.settings_title")}
            className="my-icon-btn"
          >
            <SettingsIcon />
          </button>
        )}
      </div>
    </div>
  );
};
