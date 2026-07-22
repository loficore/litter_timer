/**
 * 主页控制面板组件
 * 渲染开始/暂停/重置按钮，支持自定义动画延迟
 */

import type { FunctionalComponent } from "preact";
import { t } from "../utils/i18n";
import { Button } from "./Button";
import { PlayIconComponent, PauseIconComponent, ResetIcon } from "../utils/icons";

interface ControlPanelProps {
  /** 计时器是否正在运行 */
  isRunning: boolean;
  /** 开始按钮点击回调 */
  onStart: () => void;
  /** 暂停按钮点击回调 */
  onPause: () => void;
  /** 重置按钮点击回调 */
  onReset: () => void;
  /** 动画延迟（可选） */
  animationDelay?: string;
}

export const ControlPanel: FunctionalComponent<ControlPanelProps> = ({
  isRunning,
  onStart,
  onPause,
  onReset,
  animationDelay = "0s",
}) => {
  return (
    <div
      className="flex gap-2 sm:gap-3 md:gap-4 my-4 sm:my-6 md:my-8 justify-center flex-wrap animate-slideUp w-full"
      style={{ animationDelay, animationFillMode: "both" }}
    >
      {!isRunning ? (
        <Button variant="primary" onClick={onStart} className="flex-1 sm:flex-none sm:w-32 md:w-40">
          <PlayIconComponent />
          <span className="hidden sm:inline">{t("home.start")}</span>
        </Button>
      ) : (
        <Button variant="primary" onClick={onPause} className="flex-1 sm:flex-none sm:w-32 md:w-40">
          <PauseIconComponent />
          <span className="hidden sm:inline">{t("home.pause")}</span>
        </Button>
      )}
      <Button variant="secondary" onClick={onReset} className="flex-1 sm:flex-none sm:w-32 md:w-40">
        <ResetIcon />
        <span className="hidden sm:inline">{t("home.reset")}</span>
      </Button>
    </div>
  );
};
