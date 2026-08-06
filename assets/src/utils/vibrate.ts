export type ToastVibrationType = "success" | "info" | "error" | "timer-finish";

const TOAST_VIBRATION_PATTERNS: Record<ToastVibrationType, readonly number[]> = {
  success: [50],
  info: [50],
  error: [200, 100, 200],
  "timer-finish": [200, 100, 200],
};

const isReducedMotionPreferred = (): boolean => {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return false;
  }

  try {
    return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  } catch {
    return false;
  }
};

export const getToastVibrationPattern = (type: ToastVibrationType): readonly number[] =>
  TOAST_VIBRATION_PATTERNS[type];

export const vibrateForToast = (type: ToastVibrationType): void => {
  if (isReducedMotionPreferred()) {
    return;
  }

  if (typeof navigator === "undefined" || typeof navigator.vibrate !== "function") {
    return;
  }

  const pattern = getToastVibrationPattern(type);

  try {
    void navigator.vibrate(pattern);
  } catch {
    return;
  }
};
