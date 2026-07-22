/**
 * 数值输入组件
 * 包装 PickerNumberInput，提供标签、单位、提示与简单范围校验
 */

import { memo } from "preact/compat";
import type { FunctionalComponent } from "preact";
import { useState } from "preact/hooks";
import { PickerNumberInput } from "./PickerNumberInput";
import { t } from "../utils/i18n";

interface NumberInputProps {
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
  /** 单位文本（如 "分钟"、"%"） */
  unit?: string;
  /** 字段提示信息 */
  hint?: string;
  /** 是否禁用 */
  disabled?: boolean;
}

export const NumberInput: FunctionalComponent<NumberInputProps> = memo(({
  value,
  min = 0,
  max = 9999,
  onChange,
  label,
  unit,
  hint,
  disabled = false,
}) => {
  const [error, setError] = useState<string>("");

  const handleChange = (newValue: number) => {
    setError("");
    
    if (newValue === null || newValue === undefined) {
      setError(t("validation.input_required"));
      return;
    }

    if (min !== undefined && newValue < min) {
      onChange(min);
      return;
    }

    if (max !== undefined && newValue > max) {
      onChange(max);
      return;
    }

    onChange(newValue);
  };

  return (
    <div className="form-control w-full">
      {label && (
        <label className="label">
          <span className="label-text">{label}</span>
        </label>
      )}
      <div className="flex items-center gap-2">
        <PickerNumberInput
          value={value}
          min={min ?? 0}
          max={max ?? 9999}
          onChange={handleChange}
          disabled={disabled}
        />
        {unit && (
          <span className="label-text-alt">{unit}</span>
        )}
      </div>
      <label className="label">
        {error && <span className="label-text-alt text-error">{error}</span>}
        {!error && hint && <span className="label-text-alt">{hint}</span>}
      </label>
    </div>
  );
});

NumberInput.displayName = "NumberInput";