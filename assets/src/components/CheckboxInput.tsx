/**
 * 复选框输入组件
 * 标签与复选框同行展示，点击标签也可切换状态
 */

interface CheckboxInputProps {
  /** 当前是否勾选 */
  value: boolean;
  /** 勾选状态变化回调 */
  onChange: (checked: boolean) => void;
  /** 显示标签文本 */
  label: string;
  /** 是否禁用 */
  disabled?: boolean;
}

export const CheckboxInput = ({
  value,
  onChange,
  label,
  disabled = false,
}: CheckboxInputProps) => {
  return (
    <div className="form-control">
      <label className="label cursor-pointer gap-3">
        <input
          type="checkbox"
          checked={value}
          disabled={disabled}
          onChange={(e) => !disabled && onChange(e.currentTarget.checked)}
          className="my-checkbox"
        />
        <span className="label-text">{label}</span>
      </label>
    </div>
  );
};
