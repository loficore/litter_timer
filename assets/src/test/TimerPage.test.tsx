import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/preact";
import { TimerPage } from "../TimerPage";

const mocks = vi.hoisted(() => ({
  showToastMock: vi.fn(),
  vibrateForToastMock: vi.fn(),
  playFinishMock: vi.fn(),
  timerState: { isFinished: false },
  finishMock: vi.fn().mockResolvedValue({ elapsed_seconds: 123 }),
  resetMock: vi.fn().mockResolvedValue(undefined),
  startMock: vi.fn().mockResolvedValue(undefined),
  pauseMock: vi.fn().mockResolvedValue(undefined),
  resumeMock: vi.fn().mockResolvedValue(undefined),
  skipToNextMock: vi.fn(),
  createSessionMock: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("../utils/apiClientSingleton", () => ({
  getAPIClient: vi.fn(() => ({
    getHabitSets: vi.fn().mockResolvedValue([{ id: 1, name: "Reading" }]),
    getHabits: vi.fn().mockResolvedValue([
      { id: 7, set_id: 1, name: "Read 10 pages", color: "#ff0000", goal_seconds: 100 },
    ]),
    getHabitDetail: vi.fn().mockResolvedValue({
      habit_id: 7,
      today_seconds: 0,
      goal_seconds: 100,
    }),
    createSession: mocks.createSessionMock,
    startTimer: vi.fn().mockResolvedValue({}),
    pauseTimer: vi.fn().mockResolvedValue({}),
    resetTimer: vi.fn().mockResolvedValue({}),
    finishTimer: vi.fn().mockResolvedValue({ elapsed_seconds: 0 }),
    getTimerProgress: vi.fn().mockResolvedValue({
      session_id: null,
      is_finished: true,
      is_running: false,
      is_paused: false,
      habit_id: null,
      mode: "stopwatch",
      elapsed_seconds: 0,
      remaining_seconds: 1500,
      in_rest: false,
    }),
  })),
}));

vi.mock("../utils/audio", () => ({
  audioEngine: {
    setPreferences: vi.fn(),
    unlock: vi.fn(),
    playTick: vi.fn(),
    stopTick: vi.fn(),
    playFinish: mocks.playFinishMock,
  },
  loadAudioPreferences: vi.fn(() => ({
    sound_enabled: true,
    sound_tick: true,
    sound_finish: true,
    sound_volume: 80,
  })),
}));

vi.mock("../hooks/useSSE", () => ({
  useSSE: vi.fn(() => ({
    isConnected: false,
    lastState: null,
  })),
}));

vi.mock("../hooks/useTimer", () => ({
  useTimer: vi.fn(() => ({
    timerConfig: {
      mode: "stopwatch",
      workDuration: 1500,
      restDuration: 300,
      loopCount: 0,
    },
    setTimerConfig: vi.fn(),
    isRunning: false,
    isPaused: false,
    isFinished: mocks.timerState.isFinished,
    isResting: false,
    currentRound: 1,
    elapsedSeconds: 123,
    remainingSeconds: 1500,
    displayTime: "02:03",
    start: mocks.startMock,
    pause: mocks.pauseMock,
    resume: mocks.resumeMock,
    reset: mocks.resetMock,
    skipToNext: mocks.skipToNextMock,
    finish: mocks.finishMock,
  })),
}));

vi.mock("../utils/logger", () => ({
  logSuccess: vi.fn(),
  logError: vi.fn(),
}));

vi.mock("../utils/formatters", () => ({
  formatDuration: vi.fn((seconds: number) => {
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return `${mins.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
  }),
}));

vi.mock("../utils/i18n", () => ({
  t: (key: string, params?: Record<string, unknown>) => {
    const translations: Record<string, string> = {
      "timer.title": "Timer",
      "timer.select_habit": "Select Habit",
      "timer.stopwatch": "Stopwatch",
      "timer.countdown": "Countdown",
      "timer.start": "Start",
      "timer.pause": "Pause",
      "timer.resume": "Resume",
      "timer.reset": "Reset",
      "timer.finish": "Finish",
      "timer.skip": "Skip",
      "timer.restart": "Restart",
      "timer.resting": "Resting",
      "timer.round": "Round {current}",
      "timer.of_total": " of {total}",
      "timer.today_progress": "Today",
      "timer.goal": "Goal",
      "timer.progress": "Progress",
      "habit.no_habits": "No habits yet",
      "toast.timer_finished": "Timer finished",
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

vi.mock("../components/common/Toast", () => ({
  showToast: mocks.showToastMock,
}));

vi.mock("../utils/vibrate", () => ({
  vibrateForToast: mocks.vibrateForToastMock,
}));

describe("TimerPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.timerState.isFinished = false;
  });

  it("应该渲染计时页面", () => {
    render(<TimerPage />);
    expect(screen.getByText("Timer")).toBeTruthy();
  });

  it("应该显示标题", () => {
    render(<TimerPage />);
    expect(screen.getByText("Timer")).toBeTruthy();
  });

  it("点击导航到习惯页面", () => {
    const onHabitsClick = vi.fn();
    render(<TimerPage onHabitsClick={onHabitsClick} />);

    fireEvent.click(screen.getByRole("button", { name: "nav.habits" }));
    expect(onHabitsClick).toHaveBeenCalled();
  });

  it.skip("时间格式应该正确显示", () => {
    render(<TimerPage />);
    expect(screen.getByText("00")).toBeTruthy();
    expect(screen.getByText(":")).toBeTruthy();
  });

  it("应该显示模式选择按钮", () => {
    render(<TimerPage />);
    expect(screen.getAllByText("Stopwatch").length).toBeGreaterThan(0);
  });

  it("应该显示选择习惯按钮", () => {
    render(<TimerPage />);
    expect(screen.getAllByText("Select Habit").length).toBeGreaterThan(0);
  });

  it("在完成边缘应播放 finish 音频、toast、vibrate，并给时间显示加闪烁类", async () => {
    const { container, rerender } = render(<TimerPage />);
    const timerDisplay = container.querySelector('[data-testid="timer-display"]');

    expect(timerDisplay?.classList.contains("toast-timer-flash")).toBe(false);

    mocks.timerState.isFinished = true;
    rerender(<TimerPage />);

    await waitFor(() => {
      expect(mocks.playFinishMock).toHaveBeenCalledTimes(1);
    });

    expect(mocks.showToastMock).toHaveBeenCalledWith("Timer finished", "success");
    expect(mocks.vibrateForToastMock).toHaveBeenCalledWith("timer-finish");
    expect(timerDisplay?.classList.contains("toast-timer-flash")).toBe(true);

    mocks.timerState.isFinished = true;
    rerender(<TimerPage />);

    expect(mocks.playFinishMock).toHaveBeenCalledTimes(1);
    expect(mocks.showToastMock).toHaveBeenCalledTimes(1);
  });

  it("完成边缘只会触发一次 toast 和 vibration，即使组件再次渲染且 finished 保持为真", async () => {
    const { rerender } = render(<TimerPage />);

    mocks.timerState.isFinished = true;
    rerender(<TimerPage />);

    await waitFor(() => {
      expect(mocks.showToastMock).toHaveBeenCalledTimes(1);
      expect(mocks.vibrateForToastMock).toHaveBeenCalledWith("timer-finish");
    });

    rerender(<TimerPage />);

    expect(mocks.showToastMock).toHaveBeenCalledTimes(1);
    expect(mocks.vibrateForToastMock).toHaveBeenCalledTimes(1);
  });

  it("习惯目标达成时应记录 session 并显示习惯完成 toast", async () => {
    render(<TimerPage />);

    fireEvent.click(screen.getByTestId("timer-habit-picker"));
    fireEvent.click(await screen.findByTestId("habit-option-7"));

    fireEvent.click(screen.getByTestId("timer-start"));

    await waitFor(() => {
      expect(mocks.createSessionMock).toHaveBeenCalledWith(7, 123, 1, expect.any(String));
    });

    expect(mocks.showToastMock).toHaveBeenCalledWith("toast.habit_completed", "success");
  });
});
