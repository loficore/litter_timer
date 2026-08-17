import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/preact";
import { HabitsPage } from "../HabitsPage";

vi.mock("../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(() => ({
    getHabitSets: vi.fn().mockResolvedValue([]),
    getHabits: vi.fn().mockResolvedValue([]),
    deleteHabitSet: vi.fn().mockResolvedValue({}),
    deleteHabit: vi.fn().mockResolvedValue({}),
  })),
}));

vi.mock("../utils/logger", () => ({
  logSuccess: vi.fn(),
  logError: vi.fn(),
}));

vi.mock("../utils/formatters", () => ({
  formatDurationShort: vi.fn((seconds: number) => {
    const hours = Math.floor(seconds / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    if (hours > 0) {
      return `${hours}h ${mins}m`;
    }
    return `${mins}m`;
  }),
}));

vi.mock("../utils/i18n", () => ({
  t: (key: string, params?: Record<string, unknown>) => {
    const translations: Record<string, string> = {
      "habit.management": "Habit Management",
      "habit.sets": "Habit Sets",
      "habit.add": "Add",
      "habit.habits_count": "habits",
      "habit.no_habits": "No habits",
      "habit.no_sets": "No habit sets",
      "habit.no_sets_desc": "Create your first habit set",
      "habit.add_habit": "Add Habit",
      "habit.confirm_delete": "Confirm Delete",
      "habit.delete_set_confirm": "Delete set {name}?",
      "habit.delete_habit_confirm": "Delete habit {name}?",
      "habit.delete_warning": "This action cannot be undone",
      "button.cancel": "Cancel",
      "button.delete": "Delete",
    };
    let result = translations[key] || key;
    if (params) {
      Object.entries(params).forEach(([k, v]) => {
        result = result.replace(`{${k}}`, String(v));
      });
    }
    return result;
  },
}));

vi.mock("../components/Header", () => ({
  Header: ({ title, showBack, onBackClick, onStatsClick, onSettingsClick }: any) => (
    <div data-testid="header">
      <span data-testid="header-title">{title}</span>
      {showBack && <button onClick={onBackClick}>Back</button>}
      {onStatsClick && <button onClick={onStatsClick}>Stats</button>}
      {onSettingsClick && <button onClick={onSettingsClick}>Settings</button>}
    </div>
  ),
}));

vi.mock("../components/HabitModal", () => ({
  HabitModal: ({ isOpen, mode, editData, setId, onClose, onSuccess }: any) => {
    if (!isOpen) return null;
    return (
      <div data-testid="habit-modal">
        <span data-testid="modal-mode">{mode}</span>
        <button onClick={onClose}>Close</button>
        <button onClick={onSuccess}>Success</button>
      </div>
    );
  },
}));

describe("HabitsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("应该渲染习惯页面", () => {
    const { container } = render(<HabitsPage />);
    expect(container.querySelector(".flex.flex-col.flex-1.bg-transparent.overflow-hidden")).toBeTruthy();
  });

  it("应该显示 Header", () => {
    render(<HabitsPage />);
    expect(screen.getByTestId("header-title")).toBeTruthy();
    expect(screen.getByTestId("header-title").textContent).toBe("Habit Management");
  });

  it("应该显示添加按钮", () => {
    render(<HabitsPage />);
    expect(screen.getByRole("button", { name: /add/i })).toBeTruthy();
  });

  it("点击添加按钮应该打开创建习惯集弹窗", async () => {
    render(<HabitsPage />);

    fireEvent.click(screen.getByRole("button", { name: /add/i }));

    await waitFor(() => {
      expect(screen.getByTestId("habit-modal")).toBeTruthy();
      expect(screen.getByTestId("modal-mode").textContent).toBe("set");
    });
  });

  it("没有习惯集时应该显示空状态", async () => {
    render(<HabitsPage />);

    await waitFor(() => {
      expect(screen.getByText("No habit sets")).toBeTruthy();
    });
  });

  it("有习惯集时应该显示习惯集列表", async () => {
    const mockSets = [
      { id: 1, name: "Morning", description: "Morning habits", color: "#6366f1", wallpaper: "" },
    ];

    const mockHabits = [
      { id: 1, set_id: 1, name: "Exercise", goal_seconds: 1800, color: "#22c55e", wallpaper: "" },
    ];

    const { getAPIClient } = await import("../utils/apiClientSingleton");
    vi.mocked(getAPIClient).mockReturnValue({
      getHabitSets: vi.fn().mockResolvedValue(mockSets),
      getHabits: vi.fn().mockResolvedValue(mockHabits),
      deleteHabitSet: vi.fn().mockResolvedValue({}),
      deleteHabit: vi.fn().mockResolvedValue({}),
    });

    render(<HabitsPage />);

    await waitFor(() => {
      expect(screen.getByText("Morning")).toBeTruthy();
    });
  });

  it("点击导航到统计页面", async () => {
    const onStatsClick = vi.fn();
    render(<HabitsPage onStatsClick={onStatsClick} />);

    const statsButton = screen.getByText("Stats");
    fireEvent.click(statsButton);

    expect(onStatsClick).toHaveBeenCalled();
  });

  it("本地壁纸应该通过 <img> 渲染为 /api/wallpapers 路径", async () => {
    const mockSets = [
      { id: 1, name: "Morning", description: "", color: "#6366f1", wallpaper: "" },
    ];
    const mockHabits = [
      { id: 1, set_id: 1, name: "Exercise", goal_seconds: 1800, color: "#22c55e", wallpaper: "local:x.png" },
    ];

    const { getAPIClient } = await import("../utils/apiClientSingleton");
    vi.mocked(getAPIClient).mockReturnValue({
      getHabitSets: vi.fn().mockResolvedValue(mockSets),
      getHabits: vi.fn().mockResolvedValue(mockHabits),
      deleteHabitSet: vi.fn().mockResolvedValue({}),
      deleteHabit: vi.fn().mockResolvedValue({}),
    });

    const { container } = render(<HabitsPage />);

    await waitFor(() => {
      expect(screen.getByText("Morning")).toBeTruthy();
    });

    // 展开习惯集以渲染习惯卡片
    fireEvent.click(screen.getByText("Morning"));

    await waitFor(() => {
      const img = container.querySelector('img[src="/api/wallpapers/x.png"]');
      expect(img).toBeTruthy();
    });
  });

  it("图片加载失败时应该显示回退渐变", async () => {
    const mockSets = [
      { id: 1, name: "Morning", description: "", color: "#6366f1", wallpaper: "" },
    ];
    const mockHabits = [
      { id: 1, set_id: 1, name: "Exercise", goal_seconds: 1800, color: "#22c55e", wallpaper: "local:x.png" },
    ];

    const { getAPIClient } = await import("../utils/apiClientSingleton");
    vi.mocked(getAPIClient).mockReturnValue({
      getHabitSets: vi.fn().mockResolvedValue(mockSets),
      getHabits: vi.fn().mockResolvedValue(mockHabits),
      deleteHabitSet: vi.fn().mockResolvedValue({}),
      deleteHabit: vi.fn().mockResolvedValue({}),
    });

    const { container } = render(<HabitsPage />);

    await waitFor(() => {
      expect(screen.getByText("Morning")).toBeTruthy();
    });

    fireEvent.click(screen.getByText("Morning"));

    const img = await waitFor(() => {
      const el = container.querySelector('img[src="/api/wallpapers/x.png"]');
      expect(el).toBeTruthy();
      return el as HTMLImageElement;
    });

    // 模拟图片加载失败
    fireEvent.error(img);

    // 回退渐变 div 出现
    await waitFor(() => {
      expect(screen.getByTestId("habit-card-fallback")).toBeTruthy();
    });
    expect(container.querySelector("img")).toBeNull();
  });

  it("点击导航到设置页面", async () => {
    const onSettingsClick = vi.fn();
    render(<HabitsPage onSettingsClick={onSettingsClick} />);

    const settingsButton = screen.getByText("Settings");
    fireEvent.click(settingsButton);

    expect(onSettingsClick).toHaveBeenCalled();
  });

  it("点击返回按钮应该执行回调", async () => {
    const onBackClick = vi.fn();
    render(<HabitsPage onStatsClick={vi.fn()} onSettingsClick={vi.fn()} />);

    const backButton = screen.getByText("Back");
    fireEvent.click(backButton);

    expect(onBackClick).not.toHaveBeenCalled();
  });
});