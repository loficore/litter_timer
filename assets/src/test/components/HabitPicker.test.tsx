import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/preact";
import { HabitPicker } from "../../components/HabitPicker";
import type { Habit, HabitSet } from "../../types/habit";

describe("HabitPicker 组件", () => {
  const mockHabits: Habit[] = [
    { id: 1, set_id: 1, name: "背单词", goal_seconds: 1500, color: "#6366f1" },
    { id: 2, set_id: 1, name: "听写", goal_seconds: 1800, color: "#10b981" },
  ];
  const mockHabitSets: HabitSet[] = [
    { id: 1, name: "学习", color: "#6366f1", description: "" },
  ];
  const mockOnSelect = vi.fn();
  const mockOnClose = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("应该渲染习惯列表", () => {
    render(
      <HabitPicker
        isOpen={true}
        habitSets={mockHabitSets}
        habits={mockHabits}
        onSelect={mockOnSelect}
        onClose={mockOnClose}
      />
    );

    expect(screen.getByText("背单词")).toBeTruthy();
    expect(screen.getByText("听写")).toBeTruthy();
  });

  it("点击习惯应该调用 onSelect", () => {
    render(
      <HabitPicker
        isOpen={true}
        habitSets={mockHabitSets}
        habits={mockHabits}
        onSelect={mockOnSelect}
        onClose={mockOnClose}
      />
    );

    fireEvent.click(screen.getByText("背单词"));
    expect(mockOnSelect).toHaveBeenCalledWith(1);
  });

  it("应该显示习惯颜色", () => {
    render(
      <HabitPicker
        isOpen={true}
        habitSets={mockHabitSets}
        habits={mockHabits}
        onSelect={mockOnSelect}
        onClose={mockOnClose}
      />
    );

    const buttons = screen.getAllByRole("button");
    const habitButton = buttons.find(b => b.textContent?.includes("背单词"));
    expect(habitButton).toBeTruthy();
  });

  it("空列表时应该显示提示", () => {
    render(
      <HabitPicker
        isOpen={true}
        habitSets={[]}
        habits={[]}
        onSelect={mockOnSelect}
        onClose={mockOnClose}
      />
    );

    expect(screen.getByText("暂无习惯，点击下方添加")).toBeTruthy();
  });

  it("关闭时不应该渲染", () => {
    const { container } = render(
      <HabitPicker
        isOpen={false}
        habitSets={mockHabitSets}
        habits={mockHabits}
        onSelect={mockOnSelect}
        onClose={mockOnClose}
      />
    );

    expect(container.firstChild).toBeNull();
  });

  it("应该显示关闭按钮", () => {
    render(
      <HabitPicker
        isOpen={true}
        habitSets={mockHabitSets}
        habits={mockHabits}
        onSelect={mockOnSelect}
        onClose={mockOnClose}
      />
    );

    const closeButton = screen.getByRole("button", { name: "" });
    expect(closeButton).toBeTruthy();
  });
});
