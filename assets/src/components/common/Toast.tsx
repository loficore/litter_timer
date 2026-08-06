import type { FunctionalComponent } from "preact";
import { useCallback, useEffect, useMemo, useRef, useState } from "preact/hooks";
import { t } from "../../utils/i18n";

export type ToastType = "success" | "error" | "info";

type ToastRegion = "polite" | "assertive";

type ToastStatus = "entering" | "exiting";

const TOAST_DURATION_MS: Record<ToastType, number> = {
  success: 4000,
  info: 5000,
  error: 8000,
};

const MAX_VISIBLE_PER_REGION = 3;
const EXIT_ANIMATION_FALLBACK_MS = 240;

const getToastRegion = (type: ToastType): ToastRegion => (type === "error" ? "assertive" : "polite");

interface Toast {
  id: number;
  message: string;
  type: ToastType;
  duration: number;
  region: ToastRegion;
  status: ToastStatus;
}

interface ToastItemProps {
  toast: Toast;
  duration: number;
  onDismiss: (id: number) => void;
  onExited: (id: number) => void;
}

const supportsDesktopHover = () =>
  typeof window !== "undefined" &&
  typeof window.matchMedia === "function" &&
  window.matchMedia("(hover: hover) and (pointer: fine)").matches;

const ToastItem: FunctionalComponent<ToastItemProps> = ({ toast, duration, onDismiss, onExited }) => {
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const startedAtRef = useRef(Date.now());
  const remainingMsRef = useRef(duration);
  const hoverPauseEnabledRef = useRef(supportsDesktopHover());

  const clearAutoDismissTimer = useCallback(() => {
    if (timerRef.current !== null) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  const startAutoDismissTimer = useCallback(() => {
    if (toast.status === "exiting") return;
    clearAutoDismissTimer();
    startedAtRef.current = Date.now();
    timerRef.current = setTimeout(() => {
      onDismiss(toast.id);
    }, remainingMsRef.current);
  }, [clearAutoDismissTimer, onDismiss, toast.id, toast.status]);

  useEffect(() => {
    remainingMsRef.current = duration;
    startAutoDismissTimer();
    return clearAutoDismissTimer;
  }, [clearAutoDismissTimer, duration, startAutoDismissTimer]);

  useEffect(() => {
    if (toast.status !== "exiting") return undefined;

    clearAutoDismissTimer();
    const fallbackTimer = setTimeout(() => {
      onExited(toast.id);
    }, EXIT_ANIMATION_FALLBACK_MS);

    return () => clearTimeout(fallbackTimer);
  }, [clearAutoDismissTimer, onExited, toast.id, toast.status]);

  const pauseAutoDismiss = useCallback(() => {
    if (!hoverPauseEnabledRef.current || toast.status === "exiting") return;
    if (timerRef.current === null) return;

    const elapsedMs = Date.now() - startedAtRef.current;
    remainingMsRef.current = Math.max(0, remainingMsRef.current - elapsedMs);
    clearAutoDismissTimer();
  }, [clearAutoDismissTimer, toast.status]);

  const resumeAutoDismiss = useCallback(() => {
    if (!hoverPauseEnabledRef.current || toast.status === "exiting") return;
    startAutoDismissTimer();
  }, [startAutoDismissTimer, toast.status]);

  const handleAnimationEnd = useCallback(() => {
    if (toast.status === "exiting") {
      onExited(toast.id);
    }
  }, [onExited, toast.id, toast.status]);

  const animationClass = toast.status === "exiting" ? "lt-toast-exit" : "lt-toast-enter";

  return (
    <div
      className={`lt-toast lt-toast-${toast.type} ${animationClass}`}
      data-toast-id={toast.id}
      onAnimationEnd={handleAnimationEnd}
      onMouseEnter={pauseAutoDismiss}
      onMouseLeave={resumeAutoDismiss}
    >
      <div className="toast-float flex w-full items-center gap-3">
        <span className="min-w-0 flex-1 break-words">{toast.message}</span>
        <button
          type="button"
          className="btn btn-ghost btn-xs min-h-[var(--touch-target-min)] min-w-[var(--touch-target-min)] shrink-0 rounded-full p-0 text-current opacity-75 hover:opacity-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-current"
          aria-label={t("errors.close")}
          onClick={() => onDismiss(toast.id)}
        >
          <svg
            aria-hidden="true"
            className="h-4 w-4"
            fill="none"
            stroke="currentColor"
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth="2"
            viewBox="0 0 24 24"
          >
            <path d="M18 6 6 18" />
            <path d="m6 6 12 12" />
          </svg>
        </button>
      </div>
    </div>
  );
};

let toastIdCounter = 0;
const listeners: Set<(toast: Pick<Toast, "message" | "type">) => void> = new Set();

export const showToast = (message: string, type: ToastType = "success") => {
  listeners.forEach((listener) => listener({ message, type }));
};

export const ToastContainer: FunctionalComponent = () => {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const beginExit = useCallback((id: number) => {
    setToasts((prev) =>
      prev.map((toast) => (toast.id === id ? { ...toast, status: "exiting" } : toast)),
    );
  }, []);

  const addToast = useCallback((toast: Pick<Toast, "message" | "type">) => {
    const id = ++toastIdCounter;
    const nextToast: Toast = {
      ...toast,
      id,
      duration: TOAST_DURATION_MS[toast.type],
      region: getToastRegion(toast.type),
      status: "entering",
    };

    setToasts((prev) => {
      const next = [nextToast, ...prev];
      const visibleInRegion = next.filter((item) => item.region === nextToast.region && item.status !== "exiting");
      const overflow = visibleInRegion.slice(MAX_VISIBLE_PER_REGION);
      if (overflow.length === 0) return next;

      const overflowIds = new Set(overflow.map((item) => item.id));
      return next.map((item) => {
        if (!overflowIds.has(item.id)) return item;
        return item.status === "exiting" ? item : { ...item, status: "exiting" };
      });
    });
  }, []);

  useEffect(() => {
    listeners.add(addToast);
    return () => {
      listeners.delete(addToast);
    };
  }, [addToast]);

  const removeToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((toast) => toast.id !== id));
  }, []);

  const politeToasts = useMemo(() => toasts.filter((toast) => toast.region === "polite"), [toasts]);
  const assertiveToasts = useMemo(() => toasts.filter((toast) => toast.region === "assertive"), [toasts]);

  return (
    <div className="toast-container" aria-label="Notifications">
      <div className="flex flex-col gap-2" role="status" aria-live="polite" aria-atomic="false">
        {politeToasts.map((toast) => (
          <ToastItem
            key={toast.id}
            toast={toast}
            duration={toast.duration}
            onDismiss={beginExit}
            onExited={removeToast}
          />
        ))}
      </div>
      <div className="flex flex-col gap-2" role="alert" aria-live="assertive" aria-atomic="false">
        {assertiveToasts.map((toast) => (
          <ToastItem
            key={toast.id}
            toast={toast}
            duration={toast.duration}
            onDismiss={beginExit}
            onExited={removeToast}
          />
        ))}
      </div>
    </div>
  );
};
