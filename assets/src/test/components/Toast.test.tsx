import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/preact";
import { act } from "preact/test-utils";
import { ToastContainer, showToast } from "../../components/common/Toast";

const createMatchMedia = (matches: boolean) =>
  vi.fn().mockImplementation((query: string) => ({
    matches,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }));

describe("ToastContainer", () => {
  let matchMediaSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    vi.useFakeTimers();
    matchMediaSpy = vi.spyOn(window, "matchMedia").mockImplementation(createMatchMedia(true));
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    matchMediaSpy.mockRestore();
  });

  it("keeps a persistent shell with separate polite and assertive regions", () => {
    const { container } = render(<ToastContainer />);

    expect(container.querySelector(".toast-container")).toBeTruthy();
    expect(screen.getByRole("status")).toBeTruthy();
    expect(screen.getByRole("alert")).toBeTruthy();
  });

  it("routes error toasts into the assertive region and non-errors into the polite region", async () => {
    render(<ToastContainer />);

    showToast("saved", "success");
    showToast("failed", "error");

    await waitFor(() => {
      expect(screen.getByRole("status").textContent).toContain("saved");
      expect(screen.getByRole("alert").textContent).toContain("failed");
    });
  });

  it("pauses and resumes the auto-dismiss timer on hover-capable devices", async () => {
    render(<ToastContainer />);

    act(() => {
      showToast("hover me", "success");
    });

    const toast = await screen.findByText("hover me");
    fireEvent.mouseEnter(toast.closest(".lt-toast")!);
    act(() => {
      vi.advanceTimersByTime(3999);
    });

    expect(screen.getByText("hover me")).toBeTruthy();

    fireEvent.mouseLeave(toast.closest(".lt-toast")!);
    act(() => {
      vi.advanceTimersByTime(4000);
    });

    expect(toast.closest(".lt-toast")?.className).toContain("lt-toast-exit");

    act(() => {
      fireEvent.animationEnd(toast.closest(".lt-toast")!);
    });

    expect(screen.queryByText("hover me")).toBeNull();
  });

  it("caps visible toasts at three per region and evicts the oldest visible toast", async () => {
    render(<ToastContainer />);

    act(() => {
      showToast("one", "success");
      showToast("two", "success");
      showToast("three", "success");
      showToast("four", "success");
    });

    expect(screen.getByText("one").closest(".lt-toast")?.className).toContain("lt-toast-exit");
    expect(screen.getByText("two")).toBeTruthy();
    expect(screen.getByText("three")).toBeTruthy();
    expect(screen.getByText("four")).toBeTruthy();

    act(() => {
      vi.advanceTimersByTime(240);
    });

    expect(screen.queryByText("one")).toBeNull();
  });

  it("removes the toast when its exit animation ends and uses the fallback if animationend never fires", async () => {
    render(<ToastContainer />);

    act(() => {
      showToast("bye", "info");
    });

    const toast = await screen.findByText("bye");
    fireEvent.click(screen.getByLabelText("关闭"));

    expect(toast.closest(".lt-toast")?.className).toContain("lt-toast-exit");

    act(() => {
      vi.advanceTimersByTime(240);
    });

    expect(screen.queryByText("bye")).toBeNull();
  });

  it("falls back to timeout removal when animationend never fires", async () => {
    render(<ToastContainer />);
    act(() => {
      showToast("motion", "info");
    });

    expect(await screen.findByText("motion")).toBeTruthy();

    fireEvent.click(screen.getByLabelText("关闭"));

    expect(screen.getByText("motion").closest(".lt-toast")?.className).toContain("lt-toast-exit");

    act(() => {
      vi.advanceTimersByTime(240);
    });

    expect(screen.queryByText("motion")).toBeNull();
  });
});
