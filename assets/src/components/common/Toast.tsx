/**
 * 全局轻提示（Toast）模块
 * 通过 showToast 触发，ToastContainer 监听后渲染 3 秒自动消失
 */

import type { FunctionalComponent } from "preact";
import { useState, useEffect, useCallback } from "preact/hooks";

/** Toast 类型 */
export type ToastType = "success" | "error" | "info";

/**
 * 单条 Toast 数据
 */
interface Toast {
  /** 唯一 id */
  id: number;
  /** 提示文本 */
  message: string;
  /** 类型 */
  type: ToastType;
}

/**
 * 单条 Toast 渲染组件属性
 */
interface ToastItemProps {
  /** Toast 数据 */
  toast: Toast;
  /** 关闭回调 */
  onDismiss: (id: number) => void;
}

const ToastItem: FunctionalComponent<ToastItemProps> = ({ toast, onDismiss }) => {
  useEffect(() => {
    const timer = setTimeout(() => {
      onDismiss(toast.id);
    }, 3000);
    return () => clearTimeout(timer);
  }, [toast.id, onDismiss]);

  return (
    <div className={`toast toast-${toast.type}`} role="alert">
      {toast.message}
    </div>
  );
};

let toastIdCounter = 0;
const listeners: Set<(toast: Omit<Toast, "id">) => void> = new Set();

export const showToast = (message: string, type: ToastType = "success") => {
  listeners.forEach((listener) => listener({ message, type }));
};

export const ToastContainer: FunctionalComponent = () => {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const addToast = useCallback((toast: Omit<Toast, "id">) => {
    const id = ++toastIdCounter;
    setToasts((prev) => [...prev, { ...toast, id }]);
  }, []);

  useEffect(() => {
    listeners.add(addToast);
    return () => {
      listeners.delete(addToast);
    };
  }, [addToast]);

  const dismissToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  if (toasts.length === 0) return null;

  return (
    <div className="toast-container" role="region" aria-label="Notifications">
      {toasts.map((toast) => (
        <ToastItem key={toast.id} toast={toast} onDismiss={dismissToast} />
      ))}
    </div>
  );
};