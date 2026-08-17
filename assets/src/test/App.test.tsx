import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, cleanup } from "@testing-library/preact";
import type { ComponentChildren } from "preact";
import { App } from "../App";

const mocks = vi.hoisted(() => ({
  showToastMock: vi.fn(),
  // 服务端返回的壁纸值，测试中动态修改以驱动 App 的壁纸效果
  serverWallpaper: "",
}));

vi.mock("../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(() => ({
    getSettings: vi.fn().mockImplementation(() =>
      Promise.resolve({
        basic: {
          theme_mode: "dark",
          wallpaper: mocks.serverWallpaper,
        },
      })
    ),
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
  resolveWallpaperUrl: vi.fn((value: string) =>
    value.startsWith("local:") ? `/api/wallpapers/${value.slice("local:".length)}` : value
  ),
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

vi.mock("../WallpaperGalleryPage", () => ({
  WallpaperGalleryPage: () => <div data-testid="gallery-page" />,
}));

vi.mock("../components/ErrorBoundary", () => ({
  ErrorBoundary: ({ children, onError }: { children: ComponentChildren; onError?: (error: Error) => void }) => (
    <div data-testid="error-boundary">
      <button type="button" onClick={() => onError?.(new Error("boom"))}>trigger-error</button>
      {children}
    </div>
  ),
}));

vi.mock("../components/common/Toast", () => ({
  ToastContainer: () => <div data-testid="toast-container" />,
  showToast: mocks.showToastMock,
}));

describe("App", () => {
  // --- Image 探测桩：手动触发 onload/onerror，覆盖图片壁纸预探测两种结果 ---
  type ImageStub = {
    onload: (() => void) | null;
    onerror: (() => void) | null;
    src: string;
  };
  let imageInstances: ImageStub[];

  const installImageStub = () => {
    imageInstances = [];
    vi.stubGlobal("Image", class {
      onload: (() => void) | null = null;
      onerror: (() => void) | null = null;
      src = "";
      constructor() {
        imageInstances.push(this);
      }
    });
  };

  const fireImageLoad = (idx = 0) => {
    imageInstances[idx]?.onload?.();
  };
  const fireImageError = (idx = 0) => {
    imageInstances[idx]?.onerror?.();
  };

  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    installImageStub();
    // 清除上一个测试残留的 html 内联样式（cleanup 只卸载组件，不重置样式）
    document.documentElement.removeAttribute("style");
  });

  afterEach(() => {
    vi.unstubAllGlobals();
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

  it("点击导航到壁纸图库页面", async () => {
    render(<App />);

    const galleryButton = screen.getByTestId("nav-gallery");
    fireEvent.click(galleryButton);

    await waitFor(() => {
      expect(screen.getByTestId("gallery-page")).toBeTruthy();
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

  it("错误消息变化时应发出错误 toast，重复相同消息不重复发 toast", async () => {
    render(<App />);

    fireEvent.click(screen.getByText("trigger-error"));

    await waitFor(() => {
      expect(mocks.showToastMock).toHaveBeenCalledWith("boom", "error");
    });

    fireEvent.click(screen.getByText("trigger-error"));

    expect(mocks.showToastMock).toHaveBeenCalledTimes(1);
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

  describe("全局壁纸图片预探测", () => {
    const GRADIENT = "linear-gradient(135deg, #ff0000 0%, #0000ff 100%)";

    // 每个测试都用新组件渲染：壁纸设置经由 getSettings 异步加载
    const renderWithWallpaper = async (wp: string) => {
      mocks.serverWallpaper = wp;
      render(<App />);
      await waitFor(() => {
        expect(document.documentElement.style.backgroundImage).toBeDefined();
      });
    };

    const waitForImageProbe = async () => {
      await waitFor(() => {
        expect(imageInstances.length).toBeGreaterThan(0);
      });
    };

    it("图片壁纸 onload 成功后应用图片背景", async () => {
      await renderWithWallpaper("local:photo.jpg");
      await waitForImageProbe();

      fireImageLoad(0);

      await waitFor(() => {
        expect(document.documentElement.style.backgroundImage).toBe(
          "url(/api/wallpapers/photo.jpg)"
        );
      });
      expect(document.documentElement.style.backgroundSize).toBe("cover");
      expect(document.documentElement.style.backgroundPosition).toBe("center");
      expect(document.documentElement.style.backgroundAttachment).toBe("fixed");
    });

    it("图片壁纸 onerror 后优雅降级为回退渐变", async () => {
      await renderWithWallpaper("local:broken.jpg");
      await waitForImageProbe();

      fireImageError(0);

      // 降级路径：清除图片引用，且不设置图片长手属性（区别于 onload 成功路径）
      await waitFor(() => {
        expect(document.documentElement.style.backgroundImage).toBe("");
      });
      expect(document.documentElement.style.backgroundSize).toBe("");
      expect(document.documentElement.style.backgroundPosition).toBe("");
      expect(document.documentElement.style.backgroundAttachment).toBe("");
    });

    it("渐变/纯色壁纸不探测，直接应用", async () => {
      await renderWithWallpaper(GRADIENT);

      // 渐变分支同步持久化 wallpaper → 等待它发生以确认 effect 已执行
      await waitFor(() => {
        expect(localStorage.setItem).toHaveBeenCalledWith("global_wallpaper", GRADIENT);
      });
      // 渐变路径不创建 Image 探测实例，且不设置图片长手属性
      expect(imageInstances.length).toBe(0);
      expect(document.documentElement.style.backgroundImage).toBe("");
      expect(document.documentElement.style.backgroundSize).toBe("");
    });

    it("同 URL 图片探测结果缓存，避免重复探测", async () => {
      await renderWithWallpaper("local:cached.jpg");
      await waitForImageProbe();
      fireImageLoad(0);
      await waitFor(() => {
        expect(document.documentElement.style.backgroundImage).toBe(
          "url(/api/wallpapers/cached.jpg)"
        );
      });
      expect(imageInstances.length).toBe(1);

      // 重新挂载同一组件：命中缓存，不再创建新的 Image
      cleanup();
      mocks.serverWallpaper = "local:cached.jpg";
      render(<App />);
      await waitFor(() => {
        expect(document.documentElement.style.backgroundImage).toBe(
          "url(/api/wallpapers/cached.jpg)"
        );
      });
      expect(imageInstances.length).toBe(1);
    });
  });
});
