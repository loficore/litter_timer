import type { FunctionalComponent } from "preact";
import { useState, useEffect, useRef, useMemo } from "preact/hooks";
import { Header } from "./components/Header";
import { t } from "./utils/i18n";
import { getAPIClient } from "./utils/apiClientSingleton";
import { getToday, getDaysAgo } from "./utils/formatters";
import type { ApexOptions, ApexCharts as ApexChartsClass } from "apexcharts";
import type { Chart as ChartJsClass, ChartConfiguration } from "chart.js";
import { isPerfDebugEnabled, isWebViewRuntime, logError, logPerf } from "./utils/logger";

interface StatsPageProps {
  onBackClick: () => void;
}

interface Habit {
  id: number;
  name: string;
  color: string;
}

interface Session {
  habit_id: number;
  duration_seconds: number;
  date: string;
}

interface PerformanceMemoryInfo {
  usedJSHeapSize: number;
  totalJSHeapSize: number;
}

type TimeRange = "today" | "week" | "month" | "custom";

const parseHabitId = (raw: unknown): number | null => {
  if (typeof raw === "number") {
    return Number.isFinite(raw) ? raw : null;
  }

  if (typeof raw === "string") {
    const trimmed = raw.trim();
    if (!trimmed) return null;

    const numeric = Number(trimmed);
    return Number.isFinite(numeric) ? numeric : null;
  }

  return null;
};

const getIsWebView = (): boolean => isWebViewRuntime();

const getPerformanceMemory = (): PerformanceMemoryInfo | null => {
  const memory = (performance as Performance & { memory?: PerformanceMemoryInfo }).memory;
  return memory ?? null;
};

const formatMinutesAsHoursAndMinutes = (totalMinutes: number): string => {
  const numericMinutes = Number(totalMinutes);
  const safeMinutes = Math.max(0, Number.isFinite(numericMinutes) ? Math.floor(numericMinutes) : 0);
  const hours = Math.floor(safeMinutes / 60);
  const minutes = safeMinutes % 60;
  return `${hours}${t("common.hours")} ${minutes}${t("common.minutes")}`;
};

const formatSecondsAsHoursAndMinutes = (totalSeconds: number): string => {
  const numericSeconds = Number(totalSeconds);
  const safeSeconds = Math.max(0, Number.isFinite(numericSeconds) ? Math.floor(numericSeconds) : 0);
  const totalMinutes = Math.floor(safeSeconds / 60);
  return formatMinutesAsHoursAndMinutes(totalMinutes);
};

export const StatsPage: FunctionalComponent<StatsPageProps> = ({ onBackClick }) => {
  const [timeRange, setTimeRange] = useState<TimeRange>("week");
  const [uiTimeRange, setUiTimeRange] = useState<TimeRange>("week");
  const [habits, setHabits] = useState<Habit[]>([]);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [selectedHabitId, setSelectedHabitId] = useState<number | null>(null);
  const [uiSelectedHabitId, setUiSelectedHabitId] = useState<number | null>(null);
  
  const pieChartRef = useRef<HTMLDivElement>(null);
  const barChartCanvasRef = useRef<HTMLCanvasElement>(null);
  const pieChartInstanceRef = useRef<ApexChartsClass | null>(null);
  const barChartInstanceRef = useRef<ChartJsClass<"bar"> | null>(null);
  const ApexCtorRef = useRef<typeof ApexChartsClass | null>(null);
  const ChartJsCtorRef = useRef<typeof import("chart.js").Chart | null>(null);
  const ChartJsRegisteredRef = useRef(false);
  const isWebViewRef = useRef(getIsWebView());
  const isMountedRef = useRef(true);
  const chartRenderVersionRef = useRef(0);
  const pieLastThemeModeRef = useRef<"light" | "dark" | null>(null);
  const pieEmptyStateRef = useRef(true);
  const pieUpdateQueueRef = useRef<Promise<void>>(Promise.resolve());
  const barUpdateQueueRef = useRef<Promise<void>>(Promise.resolve());
  const statsScrollRef = useRef<HTMLDivElement>(null);
  const isScrollingRef = useRef(false);
  const scrollResumeTimerRef = useRef<number | null>(null);
  const timeRangeRafRef = useRef<number | null>(null);
  const habitFilterRafRef = useRef<number | null>(null);

  const setChartInteractionPaused = (paused: boolean) => {
    if (isScrollingRef.current === paused) return;
    isScrollingRef.current = paused;

    try {
      if (pieChartInstanceRef.current) {
        pieChartInstanceRef.current.updateOptions({
          tooltip: {
            enabled: !paused,
            followCursor: false,
          },
        }, false, false);
      }
    } catch {
      // 忽略运行期瞬态错误，下一轮渲染会覆盖。
    }

    try {
      const barChart = barChartInstanceRef.current;
      if (barChart) {
        if (barChart.options.plugins?.tooltip) {
          barChart.options.plugins.tooltip.enabled = !paused;
        }
        barChart.options.events = paused
          ? ["click", "touchstart", "touchmove"]
          : ["mousemove", "mouseout", "click", "touchstart", "touchmove"];
        barChart.update("none");
      }
    } catch {
      // 忽略运行期瞬态错误，下一轮渲染会覆盖。
    }
  };

  const enqueueChartTask = (
    queueRef: { current: Promise<void> },
    task: () => Promise<void>,
  ): Promise<void> => {
    queueRef.current = queueRef.current
      .catch(() => {
        // 吞掉上一轮异常，避免队列中断。
      })
      .then(task);
    return queueRef.current;
  };

  const chartAnimation = useMemo(() => {
    if (isWebViewRef.current) {
      return {
        enabled: true,
        easing: "easeinout" as const,
        speed: 160,
        animateGradually: { enabled: true, delay: 16 },
        dynamicAnimation: { enabled: true, speed: 120 },
      };
    }

    return {
      enabled: true,
      easing: "easeinout" as const,
      speed: 320,
      animateGradually: { enabled: true, delay: 36 },
      dynamicAnimation: { enabled: true, speed: 220 },
    };
  }, []);

  const loadApexCtor = async (): Promise<typeof ApexChartsClass> => {
    if (!ApexCtorRef.current) {
      const ApexChartsModule = await import("apexcharts");
      ApexCtorRef.current = ApexChartsModule.default;
    }
    return ApexCtorRef.current;
  };

  const loadChartJsCtor = async (): Promise<typeof import("chart.js").Chart> => {
    if (!ChartJsCtorRef.current) {
      const ChartJsModule = await import("chart.js");
      if (!ChartJsRegisteredRef.current) {
        ChartJsModule.Chart.register(...ChartJsModule.registerables);
        ChartJsRegisteredRef.current = true;
      }
      ChartJsCtorRef.current = ChartJsModule.Chart;
    }
    return ChartJsCtorRef.current;
  };

  const applyTimeRange = (range: TimeRange) => {
    setUiTimeRange((prev) => (prev === range ? prev : range));
    if (timeRangeRafRef.current !== null) {
      cancelAnimationFrame(timeRangeRafRef.current);
    }

    timeRangeRafRef.current = requestAnimationFrame(() => {
      setTimeRange((prev) => (prev === range ? prev : range));
      timeRangeRafRef.current = null;
    });
  };

  const applyHabitFilter = (habitId: number | null) => {
    setUiSelectedHabitId((prev) => (prev === habitId ? prev : habitId));
    if (habitFilterRafRef.current !== null) {
      cancelAnimationFrame(habitFilterRafRef.current);
    }

    habitFilterRafRef.current = requestAnimationFrame(() => {
      setSelectedHabitId((prev) => (prev === habitId ? prev : habitId));
      habitFilterRafRef.current = null;
    });
  };

  const loadData = async () => {
    const startAt = performance.now();
    setIsLoading(true);
    try {
      const client = getAPIClient();
      let startDate = "";
      let endDate = "";
      const today = getToday();
      
      if (timeRange === "today") {
        startDate = endDate = today;
      } else if (timeRange === "week") {
        startDate = getDaysAgo(7);
        endDate = today;
      } else if (timeRange === "month") {
        startDate = getDaysAgo(30);
        endDate = today;
      }

      const habitsStartAt = performance.now();
      const habitsData = await client.getHabits();
      const habitsDurationMs = Math.round(performance.now() - habitsStartAt);

      const sessionsStartAt = performance.now();
      const sessionsData = startDate && endDate
        ? await client.getSessions(undefined, startDate, endDate)
        : [];
      const sessionsDurationMs = Math.round(performance.now() - sessionsStartAt);

      setHabits(Array.isArray(habitsData) ? habitsData : []);
      setSessions(Array.isArray(sessionsData) ? sessionsData : []);

      const memoryInfo: Record<string, number> = {};
      const perfMemory = getPerformanceMemory();
      if (perfMemory) {
        memoryInfo.usedJSHeapSizeMB = Math.round(perfMemory.usedJSHeapSize / 1048576);
        memoryInfo.totalJSHeapSizeMB = Math.round(perfMemory.totalJSHeapSize / 1048576);
      }

      logPerf("Stats.loadData.success", {
        timeRange,
        habitsCount: Array.isArray(habitsData) ? habitsData.length : 0,
        sessionsCount: Array.isArray(sessionsData) ? sessionsData.length : 0,
        apiGetHabitsMs: habitsDurationMs,
        apiGetSessionsMs: sessionsDurationMs,
        totalDurationMs: Math.round(performance.now() - startAt),
        ...memoryInfo,
      });
    } catch (e) {
      console.error("[Stats] loadData error:", e);
      logPerf("Stats.loadData.error", {
        timeRange,
        durationMs: Math.round(performance.now() - startAt),
      });
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void loadData();
  }, [timeRange]);

  useEffect(() => {
    return () => {
      isMountedRef.current = false;
      chartRenderVersionRef.current += 1;
      if (timeRangeRafRef.current !== null) {
        cancelAnimationFrame(timeRangeRafRef.current);
      }
      if (habitFilterRafRef.current !== null) {
        cancelAnimationFrame(habitFilterRafRef.current);
      }
      pieChartInstanceRef.current?.destroy();
      barChartInstanceRef.current?.destroy();
      pieChartInstanceRef.current = null;
      barChartInstanceRef.current = null;
    };
  }, []);

  const normalizedSelectedHabitId = useMemo(() => {
    if (selectedHabitId === null) return null;
    return parseHabitId(selectedHabitId);
  }, [selectedHabitId]);

  const filteredSessions = useMemo(() => {
    if (normalizedSelectedHabitId === null) return sessions;
    return sessions.filter((s) => parseHabitId(s.habit_id) === normalizedSelectedHabitId);
  }, [sessions, normalizedSelectedHabitId]);

  const pieData = useMemo(() => {
    const canShow = normalizedSelectedHabitId === null && sessions.length > 0 && habits.length > 0;
    if (!canShow) {
      return { canShow: false, series: [] as number[], labels: [] as string[], colors: [] as string[] };
    }

    const totalsByHabit = new Map<number, number>();
    for (const session of sessions) {
      const habitId = parseHabitId(session.habit_id);
      if (habitId === null) continue;
      totalsByHabit.set(habitId, (totalsByHabit.get(habitId) || 0) + (session.duration_seconds || 0));
    }

    const validHabits = habits.filter((habit) => parseHabitId(habit.id) !== null);
    const series = validHabits.map((habit) => totalsByHabit.get(parseHabitId(habit.id) as number) || 0);
    const hasPositiveData = series.some((value) => value > 0);

    if (!hasPositiveData) {
      return { canShow: false, series: [] as number[], labels: [] as string[], colors: [] as string[] };
    }

    return {
      canShow: true,
      series,
      labels: validHabits.map((habit) => habit.name),
      colors: validHabits.map((habit) => habit.color || "#6366f1"),
    };
  }, [sessions, habits, normalizedSelectedHabitId]);

  const barData = useMemo(() => {
    const dailyMap = new Map<string, number>();
    for (const session of filteredSessions) {
      dailyMap.set(session.date, (dailyMap.get(session.date) || 0) + (session.duration_seconds || 0));
    }

    const sortedDates = Array.from(dailyMap.keys()).sort();
    const seriesData = sortedDates.map((date) => Math.round((dailyMap.get(date) || 0) / 60));

    return {
      categories: sortedDates,
      seriesData,
    };
  }, [filteredSessions]);

  useEffect(() => {
    if (isLoading) return;

    const renderVersion = chartRenderVersionRef.current + 1;
    chartRenderVersionRef.current = renderVersion;
    const isRenderActive = () => {
      return isMountedRef.current && chartRenderVersionRef.current === renderVersion;
    };

    const updateStartAt = performance.now();
    const isLightMode = typeof document !== "undefined" && document.documentElement.classList.contains("light-mode");
    const readCssVar = (name: string) => {
      if (typeof document === "undefined") return "";
      return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    };
    const chartTextColor = readCssVar("--my-on-surface") || "#f3f4f6";
    const chartMutedTextColor = readCssVar("--my-on-surface-variant") || "#9ca3af";
    const chartGridColor = readCssVar("--my-outline") || "#374151";
    const chartAccentColor = readCssVar("--accent-color") || "#6366f1";
    const chartThemeMode: "light" | "dark" = isLightMode ? "light" : "dark";

    const renderPieChart = async () => {
      const pieStartAt = performance.now();
      if (!isRenderActive()) return;

      if (!pieChartRef.current || !pieChartRef.current.isConnected) {
        // 饼图容器可能因快速切换被卸载，先销毁旧实例避免后续操作悬挂 DOM。
        pieChartInstanceRef.current?.destroy();
        pieChartInstanceRef.current = null;
        pieLastThemeModeRef.current = null;
        pieEmptyStateRef.current = true;
        return;
      }

      if (!pieData.canShow) {
        pieEmptyStateRef.current = true;
        // 无数据或筛选态下直接销毁，避免后续快速切换时在旧容器上 update。
        pieChartInstanceRef.current?.destroy();
        pieChartInstanceRef.current = null;
        pieLastThemeModeRef.current = null;
        return;
      }

      pieEmptyStateRef.current = false;

      const totalSeconds = pieData.series.reduce((sum, value) => sum + value, 0);
      const pieOptions: ApexOptions = {
        series: pieData.series,
        labels: pieData.labels,
        chart: {
          type: "donut",
          height: 300,
          background: "transparent",
          animations: chartAnimation,
        },
        colors: pieData.colors,
        plotOptions: {
          pie: {
            donut: {
              labels: {
                show: true,
                name: { show: true, color: chartTextColor },
                value: {
                  show: true,
                  color: chartTextColor,
                  formatter: (val: number) => formatSecondsAsHoursAndMinutes(val),
                },
                total: {
                  show: true,
                  label: "总计",
                  color: chartTextColor,
                  formatter: () => formatSecondsAsHoursAndMinutes(totalSeconds),
                },
              },
            },
          },
        },
        legend: { position: "bottom", labels: { colors: chartTextColor } },
        dataLabels: { enabled: false },
        stroke: { show: false },
        theme: { mode: chartThemeMode },
        tooltip: {
          theme: chartThemeMode,
          enabled: !isScrollingRef.current,
          shared: false,
          intersect: false,
          followCursor: false,
          y: {
            formatter: (val: number) => formatSecondsAsHoursAndMinutes(val),
          },
        },
      };

      if (!pieChartInstanceRef.current) {
        const ApexCharts = await loadApexCtor();
        if (!isRenderActive() || !pieChartRef.current) return;
        pieChartInstanceRef.current = new ApexCharts(pieChartRef.current, pieOptions);
        await pieChartInstanceRef.current.render();
        pieLastThemeModeRef.current = chartThemeMode;
        if (!isRenderActive()) return;
        logPerf("Stats.chart.pie.initialRender", {
          points: pieData.series.length,
          durationMs: Math.round(performance.now() - pieStartAt),
        });
        return;
      }

      if (!isRenderActive() || !pieChartRef.current.isConnected) {
        pieChartInstanceRef.current?.destroy();
        pieChartInstanceRef.current = null;
        pieLastThemeModeRef.current = null;
        pieEmptyStateRef.current = true;
        return;
      }

      try {
        if (pieLastThemeModeRef.current !== chartThemeMode || pieEmptyStateRef.current) {
          pieChartInstanceRef.current.updateOptions({
            labels: pieData.labels,
            colors: pieData.colors,
            legend: { labels: { colors: chartTextColor } },
            theme: { mode: chartThemeMode },
            tooltip: { theme: chartThemeMode },
          }, false, false);
          pieLastThemeModeRef.current = chartThemeMode;
        }
        pieChartInstanceRef.current.updateSeries(pieData.series, true);
      } catch (error) {
        // 组件切换时 ApexCharts 可能在已销毁节点上更新，重置实例避免后续连续异常。
        pieChartInstanceRef.current?.destroy();
        pieChartInstanceRef.current = null;
        pieLastThemeModeRef.current = null;
        pieEmptyStateRef.current = true;
        if (!isRenderActive()) return;
        throw error;
      }
      logPerf("Stats.chart.pie.update", {
        points: pieData.series.length,
        durationMs: Math.round(performance.now() - pieStartAt),
      });
    };

    const renderBarChart = async () => {
      const barStartAt = performance.now();
      if (!isRenderActive()) return;

      if (!barChartCanvasRef.current || !barChartCanvasRef.current.isConnected) {
        // 柱图容器在空数据或快速切换下会被卸载，旧实例必须清理。
        barChartInstanceRef.current?.destroy();
        barChartInstanceRef.current = null;
        return;
      }

      const barAnimationDuration = isWebViewRef.current ? 180 : 300;
      const barAnimationEasing = "easeOutQuart" as const;
      const barEvents: (keyof HTMLElementEventMap)[] | undefined = isWebViewRef.current
        ? (isScrollingRef.current
          ? ["click", "touchstart", "touchmove"]
          : ["mousemove", "mouseout", "click", "touchstart", "touchmove"])
        : undefined;

      if (barData.seriesData.length === 0) {
        // 对应 UI 会移除 canvas，销毁实例避免指向旧 canvas 导致不出图。
        barChartInstanceRef.current?.destroy();
        barChartInstanceRef.current = null;
        return;
      }

      if (
        barChartInstanceRef.current
        && barChartInstanceRef.current.canvas !== barChartCanvasRef.current
      ) {
        // 新旧 canvas 已替换（条件渲染），销毁旧实例强制在新容器重建。
        barChartInstanceRef.current.destroy();
        barChartInstanceRef.current = null;
      }

      if (!barChartInstanceRef.current) {
        const ChartCtor = await loadChartJsCtor();
        if (!isRenderActive() || !barChartCanvasRef.current) return;

        const config: ChartConfiguration<"bar"> = {
          type: "bar",
          data: {
            labels: barData.categories,
            datasets: [
              {
                label: t("stats.total_focus_time"),
                data: barData.seriesData,
                backgroundColor: chartAccentColor,
                borderRadius: 4,
                maxBarThickness: 42,
              },
            ],
          },
          options: {
            responsive: true,
            maintainAspectRatio: false,
            animation: {
              duration: barAnimationDuration,
              easing: barAnimationEasing,
            },
            plugins: {
              legend: {
                display: false,
                labels: { color: chartMutedTextColor },
              },
              tooltip: {
                enabled: !isScrollingRef.current,
                callbacks: {
                  label: (context) => {
                    const value = Number(context.raw || 0);
                    return `${context.dataset.label || ""}: ${formatMinutesAsHoursAndMinutes(value)}`;
                  },
                },
              },
            },
            events: barEvents,
            scales: {
              x: {
                grid: { color: chartGridColor },
                ticks: { color: chartMutedTextColor },
              },
              y: {
                grid: { color: chartGridColor },
                ticks: {
                  color: chartMutedTextColor,
                  callback: (value) => formatMinutesAsHoursAndMinutes(Number(value || 0)),
                },
              },
            },
          },
        };

        barChartInstanceRef.current = new ChartCtor(barChartCanvasRef.current, config);
        if (!isRenderActive()) return;
        logPerf("Stats.chart.bar.initialRender", {
          points: barData.seriesData.length,
          durationMs: Math.round(performance.now() - barStartAt),
        });
        return;
      }

      if (!isRenderActive() || !barChartCanvasRef.current.isConnected) {
        barChartInstanceRef.current?.destroy();
        barChartInstanceRef.current = null;
        return;
      }

      try {
        const barChart = barChartInstanceRef.current;
        barChart.data.labels = barData.categories;
        barChart.data.datasets[0].label = t("stats.total_focus_time");
        barChart.data.datasets[0].data = barData.seriesData;

        const xScale = barChart.options.scales?.x;
        const yScale = barChart.options.scales?.y;
        if (xScale) {
          xScale.grid = { color: chartGridColor };
          xScale.ticks = { color: chartMutedTextColor };
        }
        if (yScale) {
          yScale.grid = { color: chartGridColor };
          yScale.ticks = {
            color: chartMutedTextColor,
            callback: (value) => formatMinutesAsHoursAndMinutes(Number(value || 0)),
          };
        }

        barChart.update();
      } catch (error) {
        // 图表更新失败时重建实例，避免后续状态持续损坏。
        barChartInstanceRef.current?.destroy();
        barChartInstanceRef.current = null;
        if (!isRenderActive()) return;
        throw error;
      }
      logPerf("Stats.chart.bar.update", {
        points: barData.seriesData.length,
        durationMs: Math.round(performance.now() - barStartAt),
      });
    };

    void enqueueChartTask(pieUpdateQueueRef, async () => {
      if (!isRenderActive()) return;
      await renderPieChart();
    }).catch((error) => {
      if (!isRenderActive()) return;
      logError("统计页饼图更新失败", error instanceof Error ? error : undefined);
    });
    void enqueueChartTask(barUpdateQueueRef, async () => {
      if (!isRenderActive()) return;
      await renderBarChart();
    }).catch((error) => {
      if (!isRenderActive()) return;
      logError("统计页柱图更新失败", error instanceof Error ? error : undefined);
    });

    requestAnimationFrame(() => {
      if (!isRenderActive()) return;
      const memoryInfo: Record<string, number> = {};
      const perfMemory = getPerformanceMemory();
      if (perfMemory) {
        memoryInfo.usedJSHeapSizeMB = Math.round(perfMemory.usedJSHeapSize / 1048576);
        memoryInfo.totalJSHeapSizeMB = Math.round(perfMemory.totalJSHeapSize / 1048576);
      }
      logPerf("Stats.effect.frame", {
        sessions: sessions.length,
        filteredSessions: filteredSessions.length,
        totalEffectMs: Math.round(performance.now() - updateStartAt),
        ...memoryInfo,
      });
    });
  }, [isLoading, pieData, barData, chartAnimation]);

  useEffect(() => {
    if (!isPerfDebugEnabled()) return;
    logPerf("Stats.filter.changed", {
      timeRange,
      selectedHabitId,
      sessions: sessions.length,
      filtered: filteredSessions.length,
      piePoints: pieData.series.length,
      barPoints: barData.seriesData.length,
    });
  }, [timeRange, selectedHabitId, sessions.length, filteredSessions.length, pieData.series.length, barData.seriesData.length]);

  useEffect(() => {
    if (!isPerfDebugEnabled()) return;
    const scrollEl = statsScrollRef.current;
    if (!scrollEl) return;

    let rafId: number | null = null;
    let wheelStartAt = 0;

    const onWheel = () => {
      if (scrollResumeTimerRef.current !== null) {
        clearTimeout(scrollResumeTimerRef.current);
        scrollResumeTimerRef.current = null;
      }

      setChartInteractionPaused(true);
      wheelStartAt = performance.now();
      if (rafId !== null) {
        cancelAnimationFrame(rafId);
      }
      rafId = requestAnimationFrame(() => {
        const durationMs = Math.round(performance.now() - wheelStartAt);
        if (durationMs >= 24) {
          logPerf("Stats.scroll.frame", {
            durationMs,
            scrollTop: Math.round(scrollEl.scrollTop),
          });
        }
        rafId = null;
      });

      scrollResumeTimerRef.current = window.setTimeout(() => {
        setChartInteractionPaused(false);
        scrollResumeTimerRef.current = null;
      }, 180);
    };

    scrollEl.addEventListener("wheel", onWheel, { passive: true });
    return () => {
      scrollEl.removeEventListener("wheel", onWheel);
      if (rafId !== null) {
        cancelAnimationFrame(rafId);
      }
      if (scrollResumeTimerRef.current !== null) {
        clearTimeout(scrollResumeTimerRef.current);
        scrollResumeTimerRef.current = null;
      }
      setChartInteractionPaused(false);
    };
  }, []);

  const totalSeconds = useMemo(() => {
    return filteredSessions.reduce((sum, session) => sum + (session.duration_seconds || 0), 0);
  }, [filteredSessions]);

  const totalSessions = filteredSessions.length;

  return (
    <div className="flex flex-col flex-1 bg-transparent pb-16 lg:pb-0 min-h-0">
      <Header
        title={t("stats.title") || "统计"}
        showSettings={false}
        showBack={true}
        onBackClick={onBackClick}
      />

      <div ref={statsScrollRef} className="flex-1 overflow-y-auto px-4 pb-4 space-y-6 min-h-0">
        <div className="my-surface-panel flex flex-col gap-4 p-4">
          <div className="flex flex-wrap gap-2">
            {(["today", "week", "month"] as TimeRange[]).map((range) => (
              <button
                key={range}
                className={`my-filter-btn ${uiTimeRange === range ? "my-filter-btn-active" : ""}`}
                style={{ touchAction: "manipulation" }}
                onPointerDown={() => applyTimeRange(range)}
                onClick={() => applyTimeRange(range)}
              >
                {range === "today" ? t("stats.today") : range === "week" ? t("stats.this_week") : t("stats.this_month")}
              </button>
            ))}
          </div>

          <div className="h-px bg-[color:color-mix(in_oklab,var(--my-outline)_18%,transparent)]" />

          <div className="flex gap-2 overflow-x-auto pb-1">
            <button
              className={`my-filter-btn ${uiSelectedHabitId === null ? "my-filter-btn-active" : ""}`}
              style={{ touchAction: "manipulation" }}
              onPointerDown={() => applyHabitFilter(null)}
              onClick={() => applyHabitFilter(null)}
            >
              {t("stats.all")}
            </button>
            {habits.map((h) => (
              <button
                key={h.id}
                className={`my-filter-btn ${uiSelectedHabitId === h.id ? "my-filter-btn-active" : ""}`}
                style={{ touchAction: "manipulation" }}
                onPointerDown={() => applyHabitFilter(h.id)}
                onClick={() => applyHabitFilter(h.id)}
              >
                <span className="w-2 h-2 rounded-full mr-1" style={{ backgroundColor: h.color }} />
                {h.name}
              </button>
            ))}
          </div>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div className="card my-surface-card">
            <div className="card-body p-4">
              <h3 className="text-sm text-base-content/60">{t("stats.total_focus_time")}</h3>
              <p className="text-2xl font-bold text-primary">{formatSecondsAsHoursAndMinutes(totalSeconds)}</p>
            </div>
          </div>
          <div className="card my-surface-card">
            <div className="card-body p-4">
              <h3 className="text-sm text-base-content/60">{t("stats.completion_count")}</h3>
              <p className="text-2xl font-bold text-secondary">{totalSessions}</p>
            </div>
          </div>
        </div>

        {!selectedHabitId && (
          <div className="card my-surface-card">
            <div className="card-body p-4">
              <h3 className="text-lg font-semibold mb-4">{t("stats.time_distribution")}</h3>
              {isLoading ? (
                <div className="h-[300px] flex items-center justify-center">
                  <span className="loading loading-spinner loading-lg"></span>
                </div>
              ) : sessions.length === 0 ? (
                <div className="h-[300px] flex items-center justify-center text-base-content/50">
                  {t("stats.no_data")}
                </div>
              ) : (
                <div ref={pieChartRef}></div>
              )}
            </div>
          </div>
        )}

        <div className="card my-surface-card">
          <div className="card-body p-4">
            <h3 className="text-lg font-semibold mb-4">{t("stats.daily_trend")}</h3>
            {isLoading ? (
              <div className="h-[300px] flex items-center justify-center">
                <span className="loading loading-spinner loading-lg"></span>
              </div>
            ) : barData.seriesData.length === 0 ? (
              <div className="h-[300px] flex items-center justify-center text-base-content/50">
                {t("stats.no_data")}
              </div>
            ) : (
              <div className="relative h-[300px] w-full">
                <canvas ref={barChartCanvasRef} className="h-full w-full"></canvas>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};