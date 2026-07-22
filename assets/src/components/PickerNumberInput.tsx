/**
 * 数字选择器组件
 * 数字输入框 + 上下箭头按钮组合，支持键盘输入与点击步进
 */

import { memo } from "preact/compat";
import type { FunctionalComponent } from "preact";

interface PickerNumberInputProps {
  /** 当前数值 */
  value: number;
  /** 允许的最小值 */
  min?: number;
  /** 允许的最大值 */
  max?: number;
  /** 值变化回调 */
  onChange: (value: number) => void;
  /** 字段标签 */
  label?: string;
  /** 单位文本（如 "分钟"） */
  unit?: string;
  /** 字段提示信息 */
  hint?: string;
  /** 是否禁用 */
  disabled?: boolean;
  /** 输入框 data-testid */
  dataTestId?: string;
}

export const PickerNumberInput: FunctionalComponent<PickerNumberInputProps> = memo(({
  value,
  min = 0,
  max = 99,
  onChange,
  label,
  unit,
  hint,
  disabled = false,
  dataTestId,
}) => {
  const handleIncrease = () => {
    if (value < max) {
      onChange(value + 1);
    }
  };

  const handleDecrease = () => {
    if (value > min) {
      onChange(value - 1);
    }
  };

  const handleInput = (e: Event) => {
    const input = e.target as HTMLInputElement;
    const rawValue = input.value.trim();
    if (rawValue === "") return;
    
    const numValue = parseInt(rawValue, 10);
    if (Number.isNaN(numValue)) return;
    
    const clampedValue = Math.max(min, Math.min(numValue, max));
    onChange(clampedValue);
  };

  return (
    <div className="form-control w-full">
      {label && (
        <label className="label">
          <span className="label-text">{label}</span>
        </label>
      )}
      <div className="flex items-center gap-1">
        <div className="my-field-surface relative flex items-center rounded-xl h-11 overflow-hidden">
          <input
            data-testid={dataTestId}
            type="text"
            inputMode="numeric"
            value={value}
            onChange={handleInput}
            disabled={disabled}
            className={`w-16 h-full pl-3 text-center text-sm font-semibold text-[var(--my-on-surface)] bg-transparent border-none outline-none appearance-none ${
              disabled ? "opacity-50 cursor-not-allowed" : ""
            }`}
            style={{ WebkitAppearance: "none", appearance: "none" }}
          />
          <div className="flex flex-col flex-shrink-0 h-full w-6 border-l border-[color:color-mix(in_oklab,var(--my-outline)_36%,transparent)]">
            <button
              type="button"
              className="w-full flex-1 !min-w-0 !min-h-0 flex items-center justify-center text-[8px] leading-none text-[var(--my-on-surface-variant)] hover:text-[var(--my-on-surface)] hover:bg-[var(--my-primary-container)]/30 transition-colors border-b border-[color:color-mix(in_oklab,var(--my-outline)_28%,transparent)]"
              onClick={handleIncrease}
              disabled={disabled || value >= max}
            >
              ▲
            </button>
            <button
              type="button"
              className="w-full flex-1 !min-w-0 !min-h-0 flex items-center justify-center text-[8px] leading-none text-[var(--my-on-surface-variant)] hover:text-[var(--my-on-surface)] hover:bg-[var(--my-primary-container)]/30 transition-colors"
              onClick={handleDecrease}
              disabled={disabled || value <= min}
            >
              ▼
            </button>
          </div>
        </div>
        {unit && (
          <span className="text-sm text-[var(--my-on-surface-variant)] ml-1">{unit}</span>
        )}
      </div>
      {hint && (
        <label className="label">
          <span className="label-text-alt">{hint}</span>
        </label>
      )}
    </div>
  );
});

PickerNumberInput.displayName = "PickerNumberInput";