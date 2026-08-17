import { useState, useEffect, useRef } from "preact/hooks";
import type { FunctionalComponent } from "preact";
import { Header } from "./components/Header";
import { HabitModal } from "./components/HabitModal";
import { getAPIClient } from "./utils/apiClientSingleton";
import { logSuccess, logError } from "./utils/logger";
import { formatDurationShort } from "./utils/formatters";
import { t } from "./utils/i18n";
import { resolveWallpaperUrl, WALLPAPER_LOCAL_PREFIX, WALLPAPER_FALLBACK_GRADIENT, isAllowedWallpaperUrl } from "./utils/constants";
import type { HabitSet, Habit, HabitWithProgress } from "./types/habit";

interface HabitsPageProps {
    onStatsClick?: () => void;
    onSettingsClick?: () => void;
}

interface CardBackgroundProps {
    wp?: string;
}

/**
 * 卡片壁纸背景：图片类型（local: 或 http(s)）以 <img> 渲染，
 * 加载失败时降级为 WALLPAPER_FALLBACK_GRADIENT。
 * 渐变 / 纯色 / 空值返回 null（由卡片的 getCardBackgroundStyle 处理）。
 */
const CardBackground: FunctionalComponent<CardBackgroundProps> = ({ wp }) => {
    const [failed, setFailed] = useState(false);
    if (!wp) return null;
    const isImage = wp.startsWith("http") || wp.startsWith(WALLPAPER_LOCAL_PREFIX);
    if (!isImage || !isAllowedWallpaperUrl(wp)) return null;

    const src = wp.startsWith(WALLPAPER_LOCAL_PREFIX) ? resolveWallpaperUrl(wp) : wp;
    if (failed) {
        return (
            <div
                data-testid="habit-card-fallback"
                className="absolute inset-0"
                style={{ background: WALLPAPER_FALLBACK_GRADIENT }}
            />
        );
    }
    return (
        <img
            src={src}
            alt=""
            className="absolute inset-0 w-full h-full object-cover"
            onError={() => setFailed(true)}
        />
    );
};

export const HabitsPage: FunctionalComponent<HabitsPageProps> = ({
    onStatsClick,
    onSettingsClick,
}) => {
    const [habitSets, setHabitSets] = useState<HabitSet[]>([]);
    const [habits, setHabits] = useState<HabitWithProgress[]>([]);
    const [expandedSetId, setExpandedSetId] = useState<number | null>(null);
    const [modalState, setModalState] = useState<{
        isOpen: boolean;
        mode: "set" | "habit";
        editData?: HabitSet | Habit | null;
        setId?: number;
    }>({ isOpen: false, mode: "set" });
    const [deleteConfirm, setDeleteConfirm] = useState<{
        type: "set" | "habit";
        id: number;
        name: string;
    } | null>(null);

    const apiClientRef = useRef(getAPIClient());

    useEffect(() => {
        void loadData();
    }, []);

    const loadData = async () => {
        try {
            const client = apiClientRef.current;
            const sets = await client.getHabitSets();
            setHabitSets(Array.isArray(sets) ? sets : []);
            const allHabits = await client.getHabits();
            setHabits((Array.isArray(allHabits) ? allHabits : []).map((h: any) => ({
                ...h,
                today_seconds: 0,
                today_count: 0,
                progress: 0,
            })));
        } catch (e: any) {
            logError(`加载数据失败: ${e}`);
        }
    };

    const handleDelete = async () => {
        if (!deleteConfirm) return;
        try {
            const client = apiClientRef.current;
            if (deleteConfirm.type === "set") {
                await client.deleteHabitSet(deleteConfirm.id);
            } else {
                await client.deleteHabit(deleteConfirm.id);
            }
            logSuccess("✓ 删除成功");
            setDeleteConfirm(null);
            void loadData();
        } catch (e: any) {
            logError(`删除失败: ${e}`);
        }
    };

    const getHabitsBySet = (setId: number) => {
        return habits.filter((h) => h.set_id === setId);
    };

    const getCardBackgroundStyle = (habit: Habit | HabitSet): Record<string, string> => {
        const wp = habit.wallpaper;
        if (!wp) return {};

        const isGradient = wp.startsWith("linear");
        const isColor = wp.startsWith("#");

        if (isGradient) {
            return { background: wp };
        } else if (isColor) {
            return { backgroundColor: wp };
        }
        // 图片类型（local: 或 http(s)）由 CardBackground 组件以 <img> 渲染
        return {};
    };

    return (
        <div className="flex flex-col flex-1 bg-transparent overflow-hidden">
            <Header
                title={t("habit.management")}
                showSettings={false}
                showBack={true}
                onBackClick={() => {
                    // 空回调，返回时不执行任何操作
                }}
                onStatsClick={onStatsClick}
                onSettingsClick={onSettingsClick}
            />

            <div className="flex-1 overflow-y-auto p-4">
                <div className="flex justify-between items-center mb-4">
                    <h2 className="text-xl font-bold">{t("habit.sets")}</h2>
                    <button
                        className="btn btn-primary btn-sm"
                        data-testid="create-habit-set"
                        onClick={() => setModalState({ isOpen: true, mode: "set" })}
                    >
                        + {t("habit.add")}
                    </button>
                </div>

                <div className="space-y-3">
                    {habitSets.map((set) => {
                        const setHabits = getHabitsBySet(set.id);
                        const isExpanded = expandedSetId === set.id;
                        return (
                            <div key={set.id} className="my-surface-card rounded-xl overflow-hidden" data-testid="habit-set-item">
                                <div
                                    className="p-4 flex items-center justify-between cursor-pointer"
                                    onClick={() => setExpandedSetId(isExpanded ? null : set.id)}
                                >
                                    <div className="flex items-center gap-3">
                                        <div
                                            className="w-4 h-4 rounded-full"
                                            style={{ backgroundColor: set.color }}
                                        />
                                        <div>
                                            <h3 className="font-semibold">{set.name}</h3>
                                            {set.description && (
                                                <p className="text-sm text-base-content/60">{set.description}</p>
                                            )}
                                            <p className="text-xs text-base-content/50">
                                                {setHabits.length} {t("habit.habits_count")}
                                            </p>
                                        </div>
                                    </div>
                                    <div className="flex gap-2" onClick={(e: Event) => e.stopPropagation()}>
                                        <button
                                            className="btn btn-ghost btn-sm btn-circle"
                                            onClick={() => setModalState({
                                                isOpen: true,
                                                mode: "set",
                                                editData: set,
                                            })}
                                        >
                                            ✏️
                                        </button>
                                        <button
                                            className="btn btn-ghost btn-sm btn-circle"
                                            onClick={() => setDeleteConfirm({
                                                type: "set",
                                                id: set.id,
                                                name: set.name,
                                            })}
                                        >
                                            🗑️
                                        </button>
                                    </div>
                                </div>

                                {isExpanded && (
                                    <div className="border-t border-[color:color-mix(in_oklab,var(--my-outline)_28%,transparent)] p-3 space-y-2">
                                        {setHabits.length === 0 ? (
                                            <p className="text-center text-base-content/50 py-2">
                                                {t("habit.no_habits")}
                                            </p>
                                        ) : (
                                            setHabits.map((habit) => (
                                                <div
                                                    key={habit.id}
                                                    data-testid="habit-item"
                                                    className="relative my-field-surface p-3 rounded-lg flex items-center justify-between cursor-pointer transition-colors overflow-hidden"
                                                    style={getCardBackgroundStyle(habit)}
                                                >
                                                    <CardBackground wp={habit.wallpaper} />
                                                    <div className="flex items-center gap-3">
                                                        <div
                                                            className="w-3 h-3 rounded-full shrink-0"
                                                            style={{ backgroundColor: habit.color }}
                                                        />
                                                        <div>
                                                            <p className="font-medium">{habit.name}</p>
                                                            <p className="text-xs">
                                                                {t("timer.goal")}: {formatDurationShort(habit.goal_seconds)}
                                                            </p>
                                                        </div>
                                                    </div>
                                                    <div className="flex gap-2 shrink-0">
                                                        <button
                                                            className="btn btn-ghost btn-sm btn-circle"
                                                            onClick={() => setModalState({
                                                                isOpen: true,
                                                                mode: "habit",
                                                                editData: habit,
                                                                setId: set.id,
                                                            })}
                                                        >
                                                            ✏️
                                                        </button>
                                                        <button
                                                            className="btn btn-ghost btn-sm btn-circle"
                                                            onClick={() => setDeleteConfirm({
                                                                type: "habit",
                                                                id: habit.id,
                                                                name: habit.name,
                                                            })}
                                                        >
                                                            🗑️
                                                        </button>
                                                    </div>
                                                </div>
                                            ))
                                        )}
                                        <button
                                            className="btn btn-ghost btn-sm w-full mt-2"
                                            data-testid="create-habit"
                                            onClick={() => setModalState({
                                                isOpen: true,
                                                mode: "habit",
                                                setId: set.id,
                                            })}
                                        >
                                            + {t("habit.add_habit")}
                                        </button>
                                    </div>
                                )}
                            </div>
                        );
                    })}
                </div>

                {habitSets.length === 0 && (
                    <div className="text-center py-12 text-base-content/50">
                        <p>{t("habit.no_sets")}</p>
                        <p>{t("habit.no_sets_desc")}</p>
                    </div>
                )}
            </div>

            <HabitModal
                isOpen={modalState.isOpen}
                mode={modalState.mode}
                editData={modalState.editData}
                setId={modalState.setId}
                onClose={() => setModalState({ isOpen: false, mode: "set" })}
                onSuccess={() => {
                    setModalState({ isOpen: false, mode: "set" });
                    void loadData();
                }}
            />

            {deleteConfirm && (
                <div className="fixed inset-0 z-50 flex items-center justify-center">
                    <div className="my-overlay-backdrop absolute inset-0" onClick={() => setDeleteConfirm(null)} />
                    <div className="relative my-surface-modal rounded-xl p-6 w-full max-w-sm mx-4 shadow-xl">
                        <h3 className="text-lg font-bold mb-4">{t("habit.confirm_delete")}</h3>
                        <p className="mb-4">
                            {deleteConfirm.type === "set" 
                                ? t("habit.delete_set_confirm", { name: deleteConfirm.name })
                                : t("habit.delete_habit_confirm", { name: deleteConfirm.name })
                            }
                        </p>
                        <p className="text-sm text-error mb-4">{t("habit.delete_warning")}</p>
                        <div className="flex gap-2">
                            <button
                                className="btn btn-ghost flex-1"
                                onClick={() => setDeleteConfirm(null)}
                            >
                                {t("button.cancel")}
                            </button>
                            <button
                                className="btn btn-error flex-1"
                                onClick={() => void handleDelete()}
                            >
                                {t("button.delete")}
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
};
