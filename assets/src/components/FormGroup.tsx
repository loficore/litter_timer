import type { FunctionalComponent, ComponentChildren } from "preact";

/** 表单分组布局方式 */
type FormLayout = "vertical" | "horizontal";

interface FormGroupProps {
  /** 字段标签 */
  label: string;
  /** 字段提示信息 */
  hint?: string;
  /** 错误信息 */
  error?: string;
  /** 字段内容 */
  children: ComponentChildren;
  /** 布局方式 */
  layout?: FormLayout;
  /** 是否必填 */
  required?: boolean;
  /** 自定义 className */
  className?: string;
}

/**
 * 表单字段组件 - 统一管理标签、输入框、提示和错误
 *
 * @example
 * ```tsx
 * <FormGroup
 *   label="倒计时时长"
 *   hint="设置默认倒计时时长（秒）"
 *   required={true}
 *   layout="vertical"
 * >
 *   <NumberInput min={1} max={86400} {...props} />
 * </FormGroup>
 *
 * <FormGroup label="启用循环" layout="horizontal">
 *   <CheckboxInput {...props} />
 * </FormGroup>
 * ```
 */
export const FormGroup: FunctionalComponent<FormGroupProps> = ({
  label,
  hint,
  error,
  children,
  layout = "vertical",
  required = false,
  className = "",
}) => {
  const isVertical = layout === "vertical";

  return (
    <div
      className={`flex ${isVertical ? "flex-col gap-2" : "flex-row items-start gap-4"} ${className}`}
    >
      {/* 标签部分 */}
      <label
        className={`font-medium text-text-primary-dark ${
          isVertical
            ? "text-xs sm:text-sm"
            : "text-sm sm:text-base flex-shrink-0 min-w-[120px]"
        }`}
      >
        {label}
        {required && <span className="text-red-500 ml-1">*</span>}
      </label>

      {/* 内容部分 */}
      <div className={`flex-1 ${isVertical ? "w-full" : ""}`}>
        {children}

        {/* 提示和错误信息 */}
        <div className="mt-2 flex flex-col gap-1">
          {error && (
            <span className="text-xs text-red-500 font-medium">{error}</span>
          )}
          {!error && hint && (
            <span className="text-xs text-text-secondary-dark italic">
              {hint}
            </span>
          )}
        </div>
      </div>
    </div>
  );
};
