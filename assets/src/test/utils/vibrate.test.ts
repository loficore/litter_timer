import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { getToastVibrationPattern, vibrateForToast } from "../../utils/vibrate";

describe("vibrate utils", () => {
  let matchMediaSpy: ReturnType<typeof vi.spyOn>;
  let vibrateSpy: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    matchMediaSpy = vi.spyOn(window, "matchMedia").mockImplementation((query: string) => ({
      matches: query.includes("prefers-reduced-motion") ? false : true,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
    vibrateSpy = vi.fn(() => true);
    vi.stubGlobal("navigator", {
      vibrate: vibrateSpy,
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("returns the expected toast vibration pattern for finish notifications", () => {
    expect(getToastVibrationPattern("timer-finish")).toEqual([200, 100, 200]);
  });

  it("calls navigator.vibrate with the toast pattern when vibration is supported", () => {
    vibrateForToast("success");

    expect(vibrateSpy).toHaveBeenCalledWith([50]);
  });

  it("does nothing when reduced motion is preferred", () => {
    matchMediaSpy.mockImplementation((query: string) => ({
      matches: query === "(prefers-reduced-motion: reduce)",
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));

    vibrateForToast("error");

    expect(vibrateSpy).not.toHaveBeenCalled();
  });

  it("does nothing when navigator.vibrate is unavailable", () => {
    vi.unstubAllGlobals();
    vi.stubGlobal("window", {
      matchMedia: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      })),
    });
    vi.stubGlobal("navigator", {});

    expect(() => vibrateForToast("info")).not.toThrow();
  });
});
