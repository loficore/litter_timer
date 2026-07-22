/**
 * 自定义下拉选择组件
 * 不依赖原生 select，实现点击外部关闭、键盘可达的浮层菜单
 */

import { useState, useRef, useEffect } from "preact/hooks";
import type { FunctionalComponent } from "preact";
import { t } from "../utils/i18n";

/**
 * 下拉选项
 */
interface DropdownOption {
  /** 选项值 */
  value: string | number;
  /** 选项显示文本 */
  label: string;
}

interface DropdownSelectProps {
  /** 当前选中值 */
  value: string | number;
  /** 可选选项列表 */
  options: DropdownOption[];
  /** 值变化回调 */
  onChange: (value: string | number) => void;
  /** 是否禁用 */
  disabled?: boolean;
  /** 触发器与浮层的最小宽度（CSS 长度） */
  minWidth?: string;
  /** 触发器按钮的 data-testid */
  dataTestId?: string;
  /** 选项按钮的 data-testid（保留字段） */
  optionDataTestId?: string;
}

export const DropdownSelect: FunctionalComponent<DropdownSelectProps> = ({
  value,
  options,
  onChange,
  disabled = false,
  minWidth = "170px",
  dataTestId,
  optionDataTestId: _optionDataTestId,
}) => {
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // 关闭下拉菜单当点击外部时
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
      }
    };

    if (isOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      return () => {
        document.removeEventListener("mousedown", handleClickOutside);
      };
    }
  }, [isOpen]);

  const selectedOption = options.find((opt) => opt.value === value);

  const handleSelect = (optValue: string | number) => {
    onChange(optValue);
    setIsOpen(false);
  };

  return (
    <div
      ref={containerRef}
      className="relative inline-block"
      style={{ minWidth }}
    >
      {/* 选择器按钮 */}
      <button
        data-testid={dataTestId}
        className={`my-field-surface dropdown-select-btn flex items-center justify-between w-full h-11 px-4 py-3 rounded-xl transition-all duration-200 ${
          disabled ? "opacity-50 cursor-not-allowed" : "cursor-pointer"
        }`}
        style={{
          color: "var(--my-on-surface)",
          boxShadow: isOpen
            ? `inset 0 1px 0 var(--my-glass-highlight-strong), 0 0 0 2px color-mix(in oklab, var(--accent-color) 25%, transparent)`
            : "none",
        }}
        onClick={() => !disabled && setIsOpen(!isOpen)}
        disabled={disabled}
      >
        <span className="flex-1 text-left text-base">
          {selectedOption?.label || t("common.select")}
        </span>
        <svg
          xmlns="http://www.w3.org/2000/svg"
          className={`h-4 w-4 transition-transform duration-200 ${
            isOpen ? "rotate-180" : ""
          }`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>

      {/* 下拉菜单 */}
      {isOpen && (
        <div
          className="absolute top-full left-0 right-0 mt-2 my-surface-modal rounded-xl z-50 overflow-hidden transition-all duration-200"
          style={{
            boxShadow: `0 8px 24px 0 rgba(0, 0, 0, 0.2)`,
            minWidth: minWidth,
          }}
        >
          {options.map((option) => (
            <button
              key={option.value}
              className={`w-full px-3.5 py-3 text-left text-sm transition-colors duration-150 border-b border-transparent ${
                value === option.value
                  ? "bg-primary/20"
                  : "hover:bg-[color:color-mix(in_oklab,var(--my-primary-container)_42%,transparent)]"
              }`}
              style={
                value === option.value
                  ? {
                      color: "var(--accent-color)",
                      background:
                        "color-mix(in oklab, var(--accent-color) 15%, transparent)",
                    }
                  : {
                      color: "var(--my-on-surface)",
                    }
              }
              onClick={() => handleSelect(option.value)}
            >
              <div className="flex items-center justify-between">
                <span>{option.label}</span>
                {value === option.value && (
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    className="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M5 13l4 4L19 7"
                    />
                  </svg>
                )}
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  );
};
