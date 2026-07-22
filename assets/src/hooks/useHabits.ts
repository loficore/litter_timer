/**
 * 习惯数据管理 Hook
 * 统一管理习惯集、习惯和会话数据
 */

import { useState, useEffect, useRef, useCallback } from "preact/hooks";
import { getAPIClient } from "../utils/apiClientSingleton";
import { logError } from "../utils/logger";
import type { HabitSet, Habit, HabitWithProgress, HabitDetail } from "../types/habit";

/**
 * useHabits 返回的习惯数据状态和操作方法。
 */
export interface UseHabitsReturn {
  /** 当前加载的习惯集列表。 */
  habitSets: HabitSet[];
  /** 当前加载的习惯及其进度列表。 */
  habits: HabitWithProgress[];
  /** 是否正在加载习惯数据。 */
  isLoading: boolean;
  /** 最近一次加载失败的错误信息，未出错时为 null。 */
  error: string | null;

  /** 重新加载习惯集和习惯数据。 */
  refresh: () => Promise<void>;
  /**
   * 创建习惯集。
   *
   * @param name - 习惯集名称。
   * @param description - 习惯集描述。
   * @param color - 习惯集颜色。
   * @returns 创建成功的习惯集，失败时返回 null。
   */
  createSet: (name: string, description: string, color: string) => Promise<HabitSet | null>;
  /**
   * 更新习惯集。
   *
   * @param id - 习惯集 ID。
   * @param name - 更新后的名称。
   * @param description - 更新后的描述。
   * @param color - 更新后的颜色。
   * @returns 更新完成的 Promise。
   */
  updateSet: (id: number, name: string, description: string, color: string) => Promise<void>;
  /**
   * 删除习惯集。
   *
   * @param id - 要删除的习惯集 ID。
   * @returns 删除完成的 Promise。
   */
  deleteSet: (id: number) => Promise<void>;
  /**
   * 创建习惯。
   *
   * @param setId - 所属习惯集 ID。
   * @param name - 习惯名称。
   * @param goalSeconds - 目标时长，单位为秒。
   * @param color - 习惯颜色。
   * @returns 创建成功的习惯，失败时返回 null。
   */
  createHabit: (setId: number, name: string, goalSeconds: number, color: string) => Promise<Habit | null>;
  /**
   * 更新习惯。
   *
   * @param id - 习惯 ID。
   * @param name - 更新后的名称。
   * @param goalSeconds - 更新后的目标时长，单位为秒。
   * @param color - 更新后的颜色。
   * @returns 更新完成的 Promise。
   */
  updateHabit: (id: number, name: string, goalSeconds: number, color: string) => Promise<void>;
  /**
   * 删除习惯。
   *
   * @param id - 要删除的习惯 ID。
   * @returns 删除完成的 Promise。
   */
  deleteHabit: (id: number) => Promise<void>;
  /**
   * 获取指定习惯集下的习惯。
   *
   * @param setId - 习惯集 ID。
   * @returns 该习惯集下的习惯及进度列表。
   */
  getHabitsBySet: (setId: number) => HabitWithProgress[];
  /**
   * 获取习惯详情。
   *
   * @param habitId - 习惯 ID。
   * @returns 习惯详情，获取失败时返回 null。
   */
  getHabitDetail: (habitId: number) => Promise<HabitDetail | null>;
}

/**
 * 管理习惯集、习惯及其今日进度数据。
 *
 * @returns 习惯数据状态及增删改查方法。
 */
export const useHabits = (): UseHabitsReturn => {
  const apiClientRef = useRef(getAPIClient());
  const [habitSets, setHabitSets] = useState<HabitSet[]>([]);
  const [habits, setHabits] = useState<HabitWithProgress[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const setHabitSetsRef = useRef(setHabitSets);
  const setHabitsRef = useRef(setHabits);
  setHabitSetsRef.current = setHabitSets;
  setHabitsRef.current = setHabits;

  const refreshRef = useRef<() => Promise<void>>(() => Promise.reject(new Error("refresh not initialized")));

  const refresh = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const client = apiClientRef.current;
      const [setsData, habitsData] = await Promise.all([
        client.getHabitSets(),
        client.getHabits(),
      ]);
      setHabitSetsRef.current(Array.isArray(setsData) ? setsData : []);
      setHabitsRef.current(
        (Array.isArray(habitsData) ? habitsData : []).map((h: Habit) => ({
          ...h,
          today_seconds: 0,
          today_count: 0,
          progress: 0,
        }))
      );
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      logError(`加载习惯数据失败: ${msg}`);
      setError(msg);
    } finally {
      setIsLoading(false);
    }
  }, []);

  refreshRef.current = refresh;

  const createSet = useCallback(
    async (name: string, description: string, color: string): Promise<HabitSet | null> => {
      try {
        const client = apiClientRef.current;
        const newSet = await client.createHabitSet(name, description, color);
        await refreshRef.current();
        return newSet;
      } catch (e) {
        logError(`创建习惯集失败: ${e}`);
        return null;
      }
    },
    []
  );

  const updateSet = useCallback(
    async (id: number, name: string, description: string, color: string): Promise<void> => {
      try {
        const client = apiClientRef.current;
        await client.updateHabitSet(id, name, description, color);
        await refreshRef.current();
      } catch (e) {
        logError(`更新习惯集失败: ${e}`);
      }
    },
    []
  );

  const deleteSet = useCallback(
    async (id: number): Promise<void> => {
      try {
        const client = apiClientRef.current;
        await client.deleteHabitSet(id);
        await refreshRef.current();
      } catch (e) {
        logError(`删除习惯集失败: ${e}`);
      }
    },
    []
  );

  const createHabit = useCallback(
    async (
      setId: number,
      name: string,
      goalSeconds: number,
      color: string
    ): Promise<Habit | null> => {
      try {
        const client = apiClientRef.current;
        const newHabit = await client.createHabit(setId, name, goalSeconds, color);
        await refreshRef.current();
        return newHabit;
      } catch (e) {
        logError(`创建习惯失败: ${e}`);
        return null;
      }
    },
    []
  );

  const updateHabit = useCallback(
    async (
      id: number,
      name: string,
      goalSeconds: number,
      color: string
    ): Promise<void> => {
      try {
        const client = apiClientRef.current;
        await client.updateHabit(id, name, goalSeconds, color);
        await refreshRef.current();
      } catch (e) {
        logError(`更新习惯失败: ${e}`);
      }
    },
    []
  );

  const deleteHabit = useCallback(
    async (id: number): Promise<void> => {
      try {
        const client = apiClientRef.current;
        await client.deleteHabit(id);
        await refreshRef.current();
      } catch (e) {
        logError(`删除习惯失败: ${e}`);
      }
    },
    []
  );

  const getHabitsBySet = useCallback(
    (setId: number): HabitWithProgress[] => {
      return habits.filter((h) => h.set_id === setId);
    },
    [habits]
  );

  const getHabitDetail = useCallback(
    async (habitId: number): Promise<HabitDetail | null> => {
      try {
        const client = apiClientRef.current;
        const today = new Date().toISOString().split("T")[0];
        return await client.getHabitDetail(habitId, today);
      } catch (e) {
        logError(`获取习惯详情失败: ${e}`);
        return null;
      }
    },
    []
  );

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return {
    habitSets,
    habits,
    isLoading,
    error,
    refresh,
    createSet,
    updateSet,
    deleteSet,
    createHabit,
    updateHabit,
    deleteHabit,
    getHabitsBySet,
    getHabitDetail,
  };
};
