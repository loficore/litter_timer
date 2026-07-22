/**
 * 下拉选择输入组件
 * 包装 DropdownSelect，统一标签/提示样式并强制把值转为字符串
 */

import { DropdownSelect } from "./DropdownSelect";

/**
 * 下拉选项
 */
interface SelectOption {
  /** 选项值 */
  value: string | number;
  /** 选项显示文本 */
  label: string;
}

interface SelectInputProps {
  /** 当前选中值 */
  value: string | number;
  /** 可选选项列表 */
  options: SelectOption[];
  /** 值变化回调，参数统一为字符串 */
  onChange: (value: string) => void;
  /** 字段标签 */
  label?: string;
  /** 字段提示信息 */
  hint?: string;
  /** 是否禁用 */
  disabled?: boolean;
}

export const SelectInput = ({
  value,
  options,
  onChange,
  label,
  hint,
  disabled = false,
}: SelectInputProps) => {
  return (
    <div className="form-control w-full">
      {label && (
        <label className="label">
          <span className="label-text">{label}</span>
        </label>
      )}
      <DropdownSelect
        value={value}
        options={options}
        onChange={(val) => onChange(String(val))}
        disabled={disabled}
        minWidth="100%"
      />
      {hint && (
        <label className="label">
          <span className="label-text-alt">{hint}</span>
        </label>
      )}
    </div>
  );
};
