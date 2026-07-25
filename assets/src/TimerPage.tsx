import { useState, useEffect, useRef } from "preact/hooks";
import type { FunctionalComponent } from "preact";
import { getAPIClient } from "./utils/apiClientSingleton";
import { logSuccess, logError } from "./utils/logger";
import { formatDuration } from "./utils/formatters";
import { t } from "./utils/i18n";
import type { Habit, HabitSet, HabitDetail } from "./types/habit";
import {
    audioEngine,
    loadAudioPreferences,
    type AudioPreferences,
} from "./utils/audio";
import { SevenSegmentDisplay } from "./components/SevenSegmentDisplay";
import { DropdownSelect } from "./components/DropdownSelect";
import { TimerConfig as TimerConfigComponent } from "./components/TimerConfig";
import { useSSE } from "./hooks/useSSE";
import { useFinishTransition } from "./hooks/useFinishTransition";
import { useTimer } from "./hooks/useTimer";
import type { TimerMode } from "./hooks/useTimer";

interface TimerPageProps {
    onHabitsClick?: () => void;
}

type LayoutDensity = "compact" | "normal" | "spacious";
type TimeDisplayStyle = "classic" | "seven_segment";

export const TimerPage: FunctionalComponent<TimerPageProps> = ({
    onHabitsClick,
}) => {
    const [habitSets, setHabitSets] = useState<HabitSet[]>([]);
    const [habits, setHabits] = useState<Habit[]>([]);
    const [selectedHabitId, setSelectedHabitId] = useState<number | null>(null);
    const [selectedHabit, setSelectedHabit] = useState<Habit | null>(null);
    const [habitDetail, setHabitDetail] = useState<HabitDetail | null>(null);
    const [showHabitPicker, setShowHabitPicker] = useState(false);

    const [audioPreferences, setAudioPreferences] = useState<AudioPreferences>(() => loadAudioPreferences());
    const [layoutDensity, setLayoutDensity] = useState<LayoutDensity>(() => {
        const saved = localStorage.getItem("layout_density");
        return (saved as LayoutDensity) || "normal";
    });
    const [timeDisplayStyle, setTimeDisplayStyle] = useState<TimeDisplayStyle>(() => {
        const saved = localStorage.getItem("time_display_style");
        return (saved as TimeDisplayStyle) || "classic";
    });

    const sessionRecordedRef = useRef(false);
    const apiClientRef = useRef(getAPIClient());
    const previousFinishedRef = useRef(false);
    const initialDataLoadedRef = useRef(false);

    const {
        timerConfig,
        setTimerConfig,
        isRunning,
        isPaused,
        isFinished,
        isResting,
        currentRound,
        elapsedSeconds,
        remainingSeconds,
        displayTime,
        start,
        pause,
        resume,
        reset,
        skipToNext,
        finish,
    } = useTimer();

    const { isConnected: sseConnected, lastState } = useSSE();
    const { displayValue: transitionDisplayValue, isTransitioning, startTransition } = useFinishTransition(lastState);

    useEffect(() => {
        const handleSettingChange = (e: Event) => {
            const { key, value } = (e as CustomEvent).detail;
            if (key === "layout_density") {
                setLayoutDensity(value as LayoutDensity);
            }
            if (key === "time_display_style") {
                setTimeDisplayStyle(value as TimeDisplayStyle);
            }
        };

        const handleStorageChange = (e: StorageEvent) => {
            if (e.key === "layout_density" && e.newValue) {
                setLayoutDensity(e.newValue as LayoutDensity);
            }
            if (e.key === "time_display_style" && e.newValue) {
                setTimeDisplayStyle(e.newValue as TimeDisplayStyle);
            }
        };

        window.addEventListener("setting-change", handleSettingChange);
        window.addEventListener("storage", handleStorageChange);
        return () => {
            window.removeEventListener("setting-change", handleSettingChange);
            window.removeEventListener("storage", handleStorageChange);
        };
    }, []);

    useEffect(() => {
        void loadData();
    }, []);

    useEffect(() => {
        if (showHabitPicker) {
            void loadData();
        }
    }, [showHabitPicker]);

    // 当 SSE 连接成功且初始数据还未加载时，重新尝试加载数据
    useEffect(() => {
        if (sseConnected && !initialDataLoadedRef.current) {
            console.log('SSE 连接成功，重新加载数据...');
            void loadData();
        }
    }, [sseConnected]);

    useEffect(() => {
        const latest = loadAudioPreferences();
        setAudioPreferences(latest);
        audioEngine.setPreferences(latest);
    }, []);

    useEffect(() => {
        audioEngine.setPreferences(audioPreferences);
    }, [audioPreferences]);

    useEffect(() => {
        const html = document.documentElement;
        if (isRunning && !isPaused) {
            html.classList.add("breathing-blur");
        } else {
            html.classList.remove("breathing-blur");
        }
        return () => {
            html.classList.remove("breathing-blur");
        };
    }, [isRunning, isPaused]);

    useEffect(() => {
        if (selectedHabitId) {
            void loadHabitDetail(selectedHabitId);
        }
    }, [selectedHabitId]);

    useEffect(() => {
        if (isFinished && !previousFinishedRef.current) {
            audioEngine.playFinish();
        }
        previousFinishedRef.current = isFinished;
    }, [isFinished]);

    useEffect(() => {
        if (timerConfig.mode === "stopwatch" && habitDetail && elapsedSeconds > 0 && !sessionRecordedRef.current) {
            const totalTodaySeconds = habitDetail.today_seconds + elapsedSeconds;
            if (totalTodaySeconds >= habitDetail.goal_seconds) {
                sessionRecordedRef.current = true;
                void finish().then(() => {
                    void reset();
                    void recordSession();
                });
            }
        }
    }, [elapsedSeconds, habitDetail, timerConfig.mode]);

    const loadData = async () => {
        const maxRetries = 3;
        let retryCount = 0;

        const doLoadData = async (): Promise<boolean> => {
            try {
                const client = apiClientRef.current;
                const sets = await client.getHabitSets();
                setHabitSets(Array.isArray(sets) ? sets : []);
                const allHabits = await client.getHabits();
                setHabits(Array.isArray(allHabits) ? allHabits : []);
                initialDataLoadedRef.current = true;
                return true;
            } catch (e: any) {
                retryCount++;
                if (retryCount < maxRetries) {
                    // 指数退避重试：1000ms, 2000ms, 4000ms
                    const delay = 1000 * Math.pow(2, retryCount - 1);
                    console.log(`数据加载失败，${delay}ms 后进行第 ${retryCount} 次重试...`);
                    await new Promise(resolve => setTimeout(resolve, delay));
                    return doLoadData();
                } else {
                    logError(`加载数据失败，已重试 ${maxRetries} 次: ${e}`);
                    return false;
                }
            }
        };

        await doLoadData();
    };

    const loadHabitDetail = async (habitId: number) => {
        try {
            const today = new Date().toISOString().split("T")[0];
            const detail = await apiClientRef.current.getHabitDetail(habitId, today);
            setHabitDetail(detail);
            const habit = habits.find(h => h.id === habitId);
            setSelectedHabit(habit || null);
        } catch (e: any) {
            logError(`加载习惯详情失败: ${e}`);
        }
    };

    const recordSession = async () => {
        if (!selectedHabit || sessionRecordedRef.current) return;
        
        sessionRecordedRef.current = true;
        const habitId = selectedHabit.id;
        const today = new Date().toISOString().split("T")[0];
        const totalSeconds = timerConfig.mode === "stopwatch" ? elapsedSeconds : (timerConfig.workDuration * currentRound);

        try {
            await apiClientRef.current.createSession(habitId, totalSeconds, 1, today);
            logSuccess("✓ Session 已自动记录");
            void loadHabitDetail(habitId);
        } catch (e) {
            logError(`记录 session 失败: ${e}`);
        }

        if ("Notification" in window) {
            if (Notification.permission === "granted") {
                new Notification(t("notification.habit_completed"), {
                    body: t("notification.habit_completed_body", { name: selectedHabit.name })
                });
            } else if (Notification.permission !== "denied") {
                Notification.requestPermission().then((p) => {
                    if (p === "granted") {
                        new Notification(t("notification.habit_completed"), {
                            body: t("notification.habit_completed_body", { name: selectedHabit.name })
                        });
                    }
                }).catch(() => {
                    // 忽略通知权限错误
                });
            }
        }
    };

    const handleStart = async () => {
        void audioEngine.unlock();

        if (!selectedHabitId) {
            setShowHabitPicker(true);
            return;
        }

        if (isFinished) {
            await reset();
        }

        await start(selectedHabitId ?? undefined);
        audioEngine.playTick();
    };

    const handlePause = async () => {
        audioEngine.stopTick();
        try {
            await pause();
        } catch (e) {
            logError(`暂停计时失败: ${e}`);
        }
    };

    const handleResume = async () => {
        void audioEngine.unlock();
        try {
            await resume(selectedHabitId ?? undefined);
            audioEngine.playTick();
        } catch (e) {
            logError(`恢复计时失败: ${e}`);
        }
    };

    const handleFinish = async () => {
        audioEngine.stopTick();
        try {
            const result = await finish();
            logSuccess(`✓ 已计入今日统计: ${formatDuration(result.elapsed_seconds)}`);
            if (selectedHabitId) {
                void loadHabitDetail(selectedHabitId);
            }
            const localTime = timerConfig.mode === "stopwatch" ? elapsedSeconds : remainingSeconds;
            const authoritativeTime = lastState?.time ?? localTime;
            startTransition(localTime, authoritativeTime);
        } catch (e) {
            logError(`结束计时失败: ${e}`);
        }
    };

    const handleReset = async () => {
        audioEngine.stopTick();
        try {
            await reset();
        } catch (e) {
            logError(`重置计时失败: ${e}`);
        }
    };

    const handleSkipToNext = () => {
        skipToNext();
    };

    const handleHabitSelect = async (habitId: number) => {
        audioEngine.stopTick();
        void reset();
        sessionRecordedRef.current = false;

        setSelectedHabitId(habitId);
        setShowHabitPicker(false);

        try {
            const today = new Date().toISOString().split("T")[0];
            const detail = await apiClientRef.current.getHabitDetail(habitId, today);
            setHabitDetail(detail);
            const habit = habits.find(h => h.id === habitId);
            setSelectedHabit(habit || null);
        } catch (e) {
            logError(`加载习惯详情失败: ${e}`);
        }
    };

    const timeDisplay = isTransitioning ? transitionDisplayValue : displayTime;
    const timeStateClass = isFinished
        ? "time-state-finished"
        : isResting
            ? "time-state-rest"
            : "time-state-active";

    const todayProgress = habitDetail ? habitDetail.today_seconds + elapsedSeconds : elapsedSeconds;
    const progressPercent = habitDetail && habitDetail.goal_seconds > 0
        ? Math.min(Math.floor((todayProgress * 100) / habitDetail.goal_seconds), 100)
        : 0;

    return (
        <div className="flex flex-col flex-1 bg-transparent overflow-hidden">
            <header className="navbar my-topbar shrink-0 px-2 sm:px-3">
                <div className="flex-1 flex justify-center">
                    <span className="my-topbar-title text-xl font-bold">{t("timer.title")}</span>
                </div>
                <div className="flex-none">
                    <button className="my-icon-btn" onClick={onHabitsClick}>
                        <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                        </svg>
                    </button>
                </div>
            </header>

            <div className={`flex-1 w-full layout-${layoutDensity}`} style={{
                paddingLeft: 'var(--layout-px)',
                paddingRight: 'var(--layout-px)',
                paddingTop: 'var(--layout-py)',
                paddingBottom: 'var(--layout-py)',
            }}>
                <div className="mx-auto flex h-full w-full max-w-6xl flex-col justify-between">
                <div className="flex flex-wrap items-center justify-center gap-3 sm:gap-4" style={{
                    marginBottom: 'var(--layout-control-mb)',
                    gap: 'var(--layout-control-gap)',
                }}>
                    <button
                      data-testid="timer-habit-picker"
                      className="my-surface-card flex items-center gap-2 sm:gap-3 min-w-[170px] h-11 justify-between px-4 py-3 rounded-xl cursor-pointer"
                      onClick={() => setShowHabitPicker(true)}
                    >
                        {selectedHabit ? (
                            <>
                                <span className="w-3 h-3 rounded-full" style={{ backgroundColor: selectedHabit.color }} />
                                {selectedHabit.name}
                            </>
                        ) : (
                            <>
                                <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
                                </svg>
                                {t("timer.select_habit")}
                            </>
                        )}
                        <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                    </button>

                    <DropdownSelect
                        dataTestId="mode-selector"
                        value={timerConfig.mode}
                         options={[
                             { value: "stopwatch", label: t("timer.stopwatch") },
                             { value: "countdown", label: t("timer.countdown") }
                         ]}
                        onChange={(value) => setTimerConfig({...timerConfig, mode: value as TimerMode})}
                        disabled={isRunning}
                    />
                </div>

                <TimerConfigComponent
                    config={timerConfig}
                    isRunning={isRunning}
                    isCountdownMode={timerConfig.mode === "countdown"}
                    onChange={(config) => setTimerConfig({...timerConfig, ...config})}
                />

                <div className="flex flex-1 flex-col items-center justify-center">
                <div data-testid="timer-display" className="my-clock-glass mb-3 sm:mb-4">
                                    <div className={`text-[clamp(2.8rem,14vw,8rem)] leading-none font-mono font-semibold time-transition flex items-center justify-center ${timeDisplayStyle === "seven_segment" ? "time-style-seven-segment" : "time-style-classic"} ${timeStateClass} ${isRunning && timeDisplayStyle === "seven_segment" ? "time-running-segment" : ""}`}>
                       {timeDisplayStyle === "seven_segment" ? (
                            <SevenSegmentDisplay value={timeDisplay} />
                        ) : (
                            <span className="time-value-swap">
                                {timeDisplay.split(":").map((part, i, arr) => (
                                    <span key={i}>
                                        {part}
                                        {i < arr.length - 1 && <span className="time-classic-colon">:</span>}
                                    </span>
                                ))}
                            </span>
                        )}
                   </div>
                </div>

                {timerConfig.mode === "countdown" && isRunning && (
                    <div className="text-base sm:text-lg text-base-content/60 mb-6 sm:mb-7">
                        {isResting ? t("timer.resting") : t("timer.round", { current: currentRound })}
                        {timerConfig.loopCount > 0 && t("timer.of_total", { total: timerConfig.loopCount })}
                    </div>
                )}

                {habitDetail && timerConfig.mode === "stopwatch" && (
                    <div className="w-full max-w-2xl mb-6 sm:mb-8">
                        <div className="flex justify-between text-sm text-base-content/70 mb-1">
                            <span>{t("timer.today_progress")} {formatDuration(todayProgress)}</span>
                            <span>{t("timer.goal")} {formatDuration(habitDetail.goal_seconds)}</span>
                        </div>
                        <progress
                            className={`progress w-full ${isFinished ? "progress-success" : "progress-primary"}`}
                            value={Math.min(todayProgress, habitDetail.goal_seconds)}
                            max={habitDetail.goal_seconds}
                        />
                        <div className="flex justify-between items-center mt-2 text-sm">
                            <span className="text-base-content/60">
                                {t("timer.progress")} {progressPercent}%
                            </span>
                        </div>
                    </div>
                )}

                </div>

                <div className="flex flex-wrap items-center justify-center gap-3 sm:gap-4 mt-2">
                    {isRunning && !isPaused ? (
                        <button
                            data-testid="timer-pause"
                            className="btn btn-primary btn-lg min-w-[100px] sm:min-w-[130px]"
                            onClick={() => void handlePause()}
                        >
                            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 9v6m4-6v6m7-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                            </svg>
                            {t("timer.pause")}
                        </button>
                    ) : isPaused ? (
                        <button
                            data-testid="timer-resume"
                            className="btn btn-primary btn-lg min-w-[100px] sm:min-w-[130px]"
                            onClick={() => void handleResume()}
                        >
                            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                            </svg>
                            {t("timer.resume")}
                        </button>
                    ) : (
                        <button
                            data-testid="timer-start"
                            className="btn btn-primary btn-lg min-w-[100px] sm:min-w-[130px]"
                            onClick={() => void handleStart()}
                        >
                            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                            </svg>
                            {isFinished ? t("timer.restart") : (selectedHabitId ? t("timer.start") : t("timer.select_habit"))}
                        </button>
                    )}

                    {timerConfig.mode === "countdown" && isRunning && (
                        <button
                            data-testid="timer-skip"
                            className="btn btn-ghost btn-lg min-w-[90px] sm:min-w-[110px]"
                            onClick={() => void handleSkipToNext()}
                        >
                            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 5l7 7-7 7M5 5l7 7-7 7" />
                            </svg>
                            {t("timer.skip")}
                        </button>
                    )}

                    {isRunning && (
                        <button
                            data-testid="timer-finish"
                            className="btn btn-success btn-lg min-w-[90px] sm:min-w-[110px]"
                            onClick={() => void handleFinish()}
                        >
                            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                            </svg>
                            {t("timer.finish")}
                        </button>
                    )}

                    <button
                        data-testid="timer-reset"
                        className="btn btn-secondary btn-lg min-w-[90px] sm:min-w-[110px]"
                        onClick={() => void handleReset()}
                    >
                        <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                        </svg>
                        {t("timer.reset")}
                    </button>
                </div>
                </div>
            </div>

            {showHabitPicker && (
                <div className="my-overlay-backdrop fixed inset-0 z-[60] flex items-center justify-center">
                    <div className="relative my-surface-modal rounded-xl w-full max-w-md mx-4 max-h-[70vh] overflow-hidden flex flex-col">
                        <div className="p-4 border-b border-[var(--my-outline)] flex justify-between items-center">
                            <h3 className="text-lg font-bold">{t("timer.select_habit")}</h3>
                            <button className="btn btn-ghost btn-sm btn-circle" onClick={() => setShowHabitPicker(false)}>
                                <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                </svg>
                            </button>
                        </div>
                        <div className="flex-1 overflow-y-auto p-2">
                            {habitSets.length === 0 ? (
                                <div className="text-center py-8 text-[var(--my-on-surface-variant)]">
                                    <p className="mb-3">{t("habit.no_habits")}</p>
                                    {onHabitsClick && (
                                        <button className="btn btn-primary btn-sm" onClick={onHabitsClick}>
                                            {t("habit.management")}
                                        </button>
                                    )}
                                </div>
                            ) : (
                                <div className="space-y-2">
                                    {habitSets.map(set => (
                                        <div key={set.id}>
                                            <div className="px-2 py-1 text-sm font-semibold text-[var(--my-on-surface-variant)]">
                                                {set.name}
                                            </div>
                                            {habits.filter(h => h.set_id === set.id).map(habit => (
                                                <button
                                                    key={habit.id}
                                                    data-testid={`habit-option-${habit.id}`}
                                                    className="my-field-surface w-full p-3 flex items-center gap-3 rounded-lg transition-colors"
                                                    onClick={() => void handleHabitSelect(habit.id)}
                                                >
                                                    <span className="w-3 h-3 rounded-full" style={{ backgroundColor: habit.color }} />
                                                    <div className="flex-1 text-left">
                                                        <div className="font-medium">{habit.name}</div>
                                                        <div className="text-xs text-[var(--my-on-surface-variant)]">
                                                            {t("timer.goal")}: {formatDuration(habit.goal_seconds)}
                                                        </div>
                                                    </div>
                                                </button>
                                            ))}
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};
