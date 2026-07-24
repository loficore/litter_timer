import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/preact";
import { App } from "../App";

vi.mock("../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(() => ({
    getSettings: vi.fn().mockResolvedValue({
      basic: {
        theme_mode: "dark",
        wallpaper: "",
      },
    }),
  })),
}));

vi.mock("../utils/logger", () => ({
  getFrontendLogLevel: vi.fn(() => "info"),
  isPerfDebugEnabled: vi.fn(() => false),
  isWebViewRuntime: vi.fn(() => false),
  logError: vi.fn(),
  logLifecycle: vi.fn(),
  logPerf: vi.fn(),
}));

vi.mock("../utils/constants", () => ({
  WALLPAPER_FALLBACK_GRADIENT: "linear-gradient(135deg, #1a1a2e 0%, #16213e 100%)",
  STORAGE_KEYS: {
    WALLPAPER: "lt_wallpaper",
    WALLPAPER_DEBUG: "lt_wallpaper_debug",
  },
  resolveWallpaperUrl: vi.fn((value: string) => value),
}));

vi.mock("../components/Sidebar", () => ({
  Sidebar: ({ currentPage, onNavigate }: { currentPage: string; onNavigate: (p: string) => void }) => (
    <div data-testid="sidebar">
      <button onClick={() => onNavigate("timer")}>Timer</button>
      <button onClick={() => onNavigate("habits")}>Habits</button>
      <button onClick={() => onNavigate("stats")}>Stats</button>
      <button onClick={() => onNavigate("settings")}>Settings</button>
      <span data-testid="current-page">{currentPage}</span>
    </div>
  ),
}));

vi.mock("../TimerPage", () => ({
  TimerPage: ({ onHabitsClick }: { onHabitsClick?: () => void }) => (
    <div data-testid="timer-page">
      <button onClick={onHabitsClick}>Go to Habits</button>
    </div>
  ),
}));

vi.mock("../HabitsPage", () => ({
  HabitsPage: ({ onStatsClick, onSettingsClick }: { onStatsClick?: () => void; onSettingsClick?: () => void }) => (
    <div data-testid="habits-page">
      <button onClick={onStatsClick}>Go to Stats</button>
      <button onClick={onSettingsClick}>Go to Settings</button>
    </div>
  ),
}));

vi.mock("../Stats", () => ({
  StatsPage: ({ onBackClick }: { onBackClick?: () => void }) => (
    <div data-testid="stats-page">
      <button onClick={onBackClick}>Back</button>
    </div>
  ),
}));

vi.mock("../Settings", () => ({
  SettingsPage: ({ onBackClick }: { onBackClick?: () => void }) => (
    <div data-testid="settings-page">
      <button onClick={onBackClick}>Back</button>
    </div>
  ),
}));

vi.mock("../components/ErrorNotification", () => ({
  ErrorNotification: ({ visible }: { visible: boolean }) =>
    visible ? <div data-testid="error-notification">Error</div> : null,
}));

describe("App", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
  });

  it("应该渲染 App 组件", () => {
    const { getByTestId } = render(<App />);
    expect(getByTestId("sidebar")).toBeTruthy();
  });

  it("默认应该显示计时页面", () => {
    const { getByTestId } = render(<App />);
    expect(getByTestId("timer-page")).toBeTruthy();
  });

  it("应该显示底部导航", () => {
    render(<App />);
    expect(screen.getByTestId("bottom-nav")).toBeTruthy();
  });

  it("点击导航到习惯页面", async () => {
    const { getByTestId } = render(<App />);

    const habitsButton = screen.getByTestId("nav-habits");
    fireEvent.click(habitsButton);

    await waitFor(() => {
      expect(getByTestId("habits-page")).toBeTruthy();
    });
  });

  it("点击导航到设置页面", async () => {
    render(<App />);

    const settingsButton = screen.getByTestId("nav-settings");
    fireEvent.click(settingsButton);

    await waitFor(() => {
      expect(screen.getByTestId("settings-page")).toBeTruthy();
    });
  });

  it("点击导航到统计页面", async () => {
    render(<App />);

    const statsButton = screen.getByTestId("nav-stats");
    fireEvent.click(statsButton);

    await waitFor(() => {
      expect(screen.getByTestId("stats-page")).toBeTruthy();
    });
  });

  it("从设置页面返回应该显示计时页面", async () => {
    render(<App />);

    fireEvent.click(screen.getByTestId("nav-settings"));
    await waitFor(() => {
      expect(screen.getByTestId("settings-page")).toBeTruthy();
    });

    const backButton = screen.getByText("Back");
    fireEvent.click(backButton);

    await waitFor(() => {
      expect(screen.getByTestId("timer-page")).toBeTruthy();
    });
  });

  it("navigateTo 函数应该正确更新页面", async () => {
    const { getByTestId, rerender } = render(<App />);

    expect(getByTestId("current-page").textContent).toBe("timer");

    fireEvent.click(screen.getByText("Habits"));
    await waitFor(() => {
      expect(getByTestId("current-page").textContent).toBe("habits");
    });
  });

  it("TimerPage 的 onHabitsClick 应该导航到习惯页面", async () => {
    render(<App />);

    const goToHabitsButton = screen.getByText("Go to Habits");
    fireEvent.click(goToHabitsButton);

    await waitFor(() => {
      expect(screen.getByTestId("habits-page")).toBeTruthy();
    });
  });

  it("HabitsPage 的 onSettingsClick 应该导航到设置页面", async () => {
    render(<App />);

    fireEvent.click(screen.getByTestId("nav-habits"));
    await waitFor(() => {
      expect(screen.getByTestId("habits-page")).toBeTruthy();
    });

    const goToSettingsButton = screen.getByText("Go to Settings");
    fireEvent.click(goToSettingsButton);

    await waitFor(() => {
      expect(screen.getByTestId("settings-page")).toBeTruthy();
    });
  });
});
