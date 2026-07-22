/**
 * 错误边界组件，用于捕获子组件的 JavaScript 错误
 * 同文件还导出 PageErrorBoundary 页面级错误边界（带自定义 fallback）
 */

import type { FunctionalComponent, ComponentChildren } from "preact";
import { Component } from "preact";
import { logError } from "../utils/logger";
import { t } from "../utils/i18n";

/**
 * ErrorBoundary 组件属性
 */
interface ErrorBoundaryProps {
  /** 子节点 */
  children: ComponentChildren;
  /** 自定义错误回退 UI */
  fallback?: ComponentChildren;
  /** 错误捕获回调，用于上报或自定义处理 */
  onError?: (error: Error, errorInfo: { componentStack?: string }) => void;
}

/**
 * ErrorBoundary 组件状态
 */
interface ErrorBoundaryState {
  /** 是否已捕获到错误 */
  hasError: boolean;
  /** 捕获的错误对象 */
  error: Error | null;
}

/**
 * 错误边界组件，用于捕获子组件的 JavaScript 错误
 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  /**
   * 构造函数
   */
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  /**
   * 从错误派生新状态
   */
  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error };
  }

  /**
   * 捕获错误并记录日志
   */
  componentDidCatch(error: Error, errorInfo: { componentStack?: string }): void {
    logError("ErrorBoundary caught an error", error);
    logError(`Component stack: ${errorInfo?.componentStack || "no stack"}`);
    this.props.onError?.(error, errorInfo);
  }

  /**
   * 渲染错误边界内容
   */
  render(): ComponentChildren {
    if (this.state.hasError && this.state.error) {
      if (this.props.fallback) {
        return this.props.fallback;
      }

      return (
        <div className="flex flex-col items-center justify-center min-h-[200px] p-8 text-center">
          <div className="text-error text-4xl mb-4">⚠️</div>
          <h2 className="text-xl font-bold text-error mb-2">{t("errors.error_occurred")}</h2>
          <p className="text-base-content/70 mb-4 max-w-md">
            {this.state.error.message || t("errors.unknown_error")}
          </p>
           <button
             className="btn btn-primary"
             onClick={() => window.location.reload()}
           >
             {t("errors.reload")}
           </button>
        </div>
      );
    }

    return this.props.children;
  }
}

/**
 * PageErrorBoundary 组件属性
 */
interface PageErrorBoundaryProps {
  /** 页面内容 */
  children: ComponentChildren;
  /** 自定义错误页标题 */
  title?: string;
}

/**
 * 页面级错误边界组件，提供默认的错误 UI
 */
export const PageErrorBoundary: FunctionalComponent<PageErrorBoundaryProps> = ({
  children,
   title = t("errors.page_load_failed"),
}) => {
  return (
    <ErrorBoundary
      fallback={
        <div className="flex flex-col items-center justify-center min-h-[300px] p-8 text-center">
          <div className="text-warning text-5xl mb-4">⚠️</div>
          <h2 className="text-xl font-bold text-warning mb-2">{title}</h2>
          <p className="text-base-content/60 mb-6 max-w-md">
            页面加载时发生错误，请尝试重新加载
          </p>
          <button
            className="btn btn-primary"
            onClick={() => window.location.reload()}
          >
            重新加载页面
          </button>
        </div>
      }
    >
      {children}
    </ErrorBoundary>
  );
};
