/**
 * 习惯选择器组件
 * 用于选择要计时的习惯
 */

import { memo } from "preact/compat";
import type { FunctionalComponent } from "preact";
import { formatDuration } from "../utils/formatters";
import { t } from "../utils/i18n";
import type { Habit, HabitSet } from "../types/habit";

interface HabitPickerProps {
  /** 是否显示弹窗 */
  isOpen: boolean;
  /** 习惯分组列表 */
  habitSets: HabitSet[];
  /** 全部习惯列表 */
  habits: Habit[];
  /** 关闭弹窗回调 */
  onClose: () => void;
  /** 选中某个习惯的回调 */
  onSelect: (habitId: number) => void;
}

export const HabitPicker: FunctionalComponent<HabitPickerProps> = memo(({
  isOpen,
  habitSets,
  habits,
  onClose,
  onSelect,
}) => {
  if (!isOpen) {
    return null;
  }

  return (
    <div className="my-overlay-backdrop fixed inset-0 z-[60] flex items-center justify-center">
      <div className="relative my-surface-modal rounded-xl w-full max-w-md mx-4 max-h-[70vh] overflow-hidden flex flex-col">
        <div className="p-4 border-b border-[var(--my-outline)] flex justify-between items-center">
          <h3 className="text-lg font-bold">{t("habit.picker_title")}</h3>
          <button className="btn btn-ghost btn-sm btn-circle" onClick={onClose}>
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        <div className="flex-1 overflow-y-auto p-2">
          {habitSets.length === 0 ? (
            <div className="text-center py-8 text-[var(--my-on-surface-variant)]">
              <p>{t("habit.no_habits")}</p>
            </div>
          ) : (
            <div className="space-y-2">
              {habitSets.map(set => (
                <div key={set.id}>
                  <div className="px-2 py-1 text-sm font-semibold text-[var(--my-on-surface-variant)]">
                    {set.name}
                  </div>
                  {habits.filter(h => h.set_id === set.id).map(habit => (
                    <button
                      key={habit.id}
                      data-testid={`habit-option-${habit.id}`}
                      className="my-filter-btn w-full flex items-center gap-3 justify-start text-left"
                      onClick={() => onSelect(habit.id)}
                    >
                      <span className="w-3 h-3 rounded-full" style={{ backgroundColor: habit.color }} />
                      <div className="flex-1 text-left">
                        <div className="font-medium">{habit.name}</div>
                        <div className="text-xs text-[var(--my-on-surface-variant)]">
                          目标: {formatDuration(habit.goal_seconds)}
                        </div>
                      </div>
                    </button>
                  ))}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
});

HabitPicker.displayName = "HabitPicker";