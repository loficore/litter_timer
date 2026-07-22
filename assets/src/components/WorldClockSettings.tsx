/**
 * 世界时钟设置组件
 * 提供时区下拉选择，用于世界时钟模式显示对应时区时间
 */

import { SettingItem } from "./SettingItem";
import { SelectInput } from "./SelectInput";
import { t } from "../utils/i18n";

interface WorldClockSettingsProps {
  /** 当前时区偏移量 */
  timezone: number;
  /** 时区变化回调 */
  onTimezoneChange: (tz: number) => void;
}

export const WorldClockSettings = ({
  timezone,
  onTimezoneChange,
}: WorldClockSettingsProps) => {
  const timezones = Array.from({ length: 27 }, (_, i) => i - 12).map((tz) => ({
    value: tz,
    label: `UTC${tz >= 0 ? "+" : ""}${tz}`,
  }));

  return (
    <div
      className="space-y-4 sm:space-y-6 animate-slideUp"
      style={{ animationDelay: "0.3s", animationFillMode: "both" }}
    >
      <SettingItem label={t("settings.world_clock.timezone")}>
        <SelectInput
          value={timezone}
          options={timezones}
          onChange={(value) => onTimezoneChange(parseInt(value))}
          hint={t("settings.basic.timezone_hint", {
            offset: `${timezone >= 0 ? "+" : ""}${timezone}`,
          })}
        />
      </SettingItem>
      <div className="text-xs sm:text-sm text-text-secondary-dark leading-relaxed">
        {t("settings.world_clock.desc")}
      </div>
    </div>
  );
};
