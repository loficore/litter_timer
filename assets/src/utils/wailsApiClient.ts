/**
 * Platform-aware API client for Little Timer.
 *
 * On Android (detected via window.wails): uses Wails v3 JS↔Go bindings
 * via JNI — no HTTP fetch needed.
 *
 * On desktop: uses the existing fetch()-based APIClient.
 *
 * This allows the same frontend code to work on both platforms
 * without changing call sites — they keep using getAPIClient().
 */

import type {
  TimerState,
  TimerProgress,
  TimerStartOptions,
  TimerStartResult,
  TimerFinishResult,
  RestResult,
  ResumeResult,
  HabitSet,
  Habit,
  HabitDetail,
  Session,
  CreateSessionResult,
  BackupConfig,
  BackupListResult,
  BackupCreateResult,
  BackupRestoreResult,
  BackupVerifyResult,
  WallpaperUploadResult,
  WallpaperListResult,
  WallpaperDeleteResult,
} from "../types/api";

/* eslint-disable @typescript-eslint/require-await */
// Detect Android: Wails v3 sets window.wails on Android
const isAndroid = typeof window !== "undefined" && !!(window as any).wails;

// ponytail: lazy-load Wails bindings only on Android at runtime
let _bindings: {
  TimerService: any;
  SettingsService: any;
  HabitService: any;
  BackupService: any;
} | null = null;

async function _loadBindings() {
  if (_bindings) return _bindings;
  // Use string-based dynamic import to prevent Rollup from resolving at build time.
  // These files are Wails-generated (Android only) and don't exist in the desktop build.
  const [ts, ss, hs, bs] = await Promise.all([
    import(/* @vite-ignore */ "../bindings/little-timer/internal/app/timerservice.ts"),
    import(/* @vite-ignore */ "../bindings/little-timer/internal/app/settingsservice.ts"),
    import(/* @vite-ignore */ "../bindings/little-timer/internal/app/habitservice.ts"),
    import(/* @vite-ignore */ "../bindings/little-timer/internal/app/backupservice.ts"),
  ]);
  _bindings = {
    TimerService: ts,
    SettingsService: ss,
    HabitService: hs,
    BackupService: bs,
  };
  return _bindings;
}
/**
 * Wails API 客户端 — 封装所有与后端通信的 Wails Go 绑定方法
 */
export class WailsAPIClient {
  // Exposed for useSSE which accesses apiClient.baseUrl — SSE is not used on Android
  baseUrl = "";

  private async _b(name: 'TimerService' | 'SettingsService' | 'HabitService' | 'BackupService') {
    const b = await _loadBindings();
    return b[name];
  }

  /**
   * 获取当前计时器状态
   */
  async getState(): Promise<TimerState> {
    const TS = await this._b('TimerService');
    return TS.GetState();
  }

  /**
   * 启动计时器
   * @param habitId - 可选习惯 ID
   * @param options - 可选计时配置（模式、时长等）
   */
  async startTimer(habitId?: number, options?: TimerStartOptions): Promise<TimerStartResult> {
    const TS = await this._b('TimerService');
    return TS.StartTimer(
      habitId ?? null,
      options?.mode ?? "",
      options?.workDuration ?? 0,
      options?.restDuration ?? 0,
      options?.loopCount ?? 0
    );
  }

  /**
   * 结束计时器
   */
  async finishTimer(): Promise<TimerFinishResult> {
    const TS = await this._b('TimerService');
    return TS.FinishTimer();
  }

  /**
   * 获取计时器进度详情
   */
  async getTimerProgress(): Promise<TimerProgress> {
    const TS = await this._b('TimerService');
    return TS.GetProgress();
  }

  /**
   * 恢复计时器（等同于 startTimer）
   */
  async resumeTimer(habitId?: number): Promise<ResumeResult> {
    return this.startTimer(habitId);
  }

  /**
   * 开始休息时段
   */
  async startRest(): Promise<RestResult> {
    const TS = await this._b('TimerService');
    return TS.StartRest();
  }

  /**
   * 暂停计时器
   */
  async pauseTimer(): Promise<void> {
    const TS = await this._b('TimerService');
    return TS.PauseTimer();
  }

  /**
   * 重置计时器
   */
  async resetTimer(): Promise<void> {
    const TS = await this._b('TimerService');
    return TS.ResetTimer();
  }

  /**
   * 切换计时模式（仅桌面端，Android 无效）
   */
  async changeMode(_mode: "countdown" | "stopwatch"): Promise<void> {
    // Wails bindings don't expose changeMode — this is a no-op on Android
    // since mode is set via StartTimer options
    return Promise.resolve();
  }

  /**
   * 获取应用设置
   */
  async getSettings(): Promise<any> {
    const SS = await this._b('SettingsService');
    return SS.GetSettings();
  }

  /**
   * 更新应用设置
   * @param settings - 设置对象
   */
  async updateSettings(settings: object): Promise<void> {
    const SS = await this._b('SettingsService');
    return SS.UpdateSettings(JSON.stringify(settings));
  }

  /**
   * 获取所有习惯集
   */
  async getHabitSets(): Promise<HabitSet[]> {
    const HS = await this._b('HabitService');
    return HS.ListHabitSets();
  }

  /**
   * 创建新习惯集
   * @param name - 名称
   * @param description - 描述
   * @param color - 颜色
   */
  async createHabitSet(name: string, description: string, color: string): Promise<HabitSet> {
    const HS = await this._b('HabitService');
    return HS.CreateHabitSet(name, description, color);
  }

  /**
   * 更新习惯集
   * @param id - 习惯集 ID
   * @param name - 名称
   * @param description - 描述
   * @param color - 颜色
   * @param wallpaper - 可选壁纸
   */
  async updateHabitSet(id: number, name: string, description: string, color: string, wallpaper?: string): Promise<HabitSet> {
    const HS = await this._b('HabitService');
    return HS.UpdateHabitSet(id, name, description, color, wallpaper ?? "");
  }

  /**
   * 删除习惯集
   * @param id - 习惯集 ID
   */
  async deleteHabitSet(id: number): Promise<void> {
    const HS = await this._b('HabitService');
    return HS.DeleteHabitSet(id);
  }

  /**
   * 获取所有习惯
   */
  async getHabits(): Promise<Habit[]> {
    const HS = await this._b('HabitService');
    return HS.ListHabits(null);
  }

  /**
   * 创建新习惯
   * @param setId - 所属习惯集 ID
   * @param name - 名称
   * @param goalSeconds - 目标秒数
   * @param color - 颜色
   */
  async createHabit(setId: number, name: string, goalSeconds: number, color: string): Promise<Habit> {
    const HS = await this._b('HabitService');
    return HS.CreateHabit(setId, name, goalSeconds, color);
  }

  /**
   * 删除习惯
   * @param id - 习惯 ID
   */
  async deleteHabit(id: number): Promise<void> {
    const HS = await this._b('HabitService');
    return HS.DeleteHabit(id);
  }

  /**
   * 更新习惯
   * @param id - 习惯 ID
   * @param name - 名称
   * @param goalSeconds - 目标秒数
   * @param color - 颜色
   * @param wallpaper - 可选壁纸
   */
  async updateHabit(id: number, name: string, goalSeconds: number, color: string, wallpaper?: string): Promise<Habit> {
    const HS = await this._b('HabitService');
    return HS.UpdateHabit(id, name, goalSeconds, color, wallpaper ?? "");
  }

  /**
   * 记录一次习惯完成
   * @param habitId - 习惯 ID
   * @param durationSeconds - 实际时长（秒）
   * @param count - 完成次数
   * @param date - 日期字符串
   */
  async createSession(habitId: number, durationSeconds: number, count: number, date: string): Promise<CreateSessionResult> {
    const HS = await this._b('HabitService');
    return HS.CreateSession(habitId, durationSeconds, count, date);
  }

  /**
   * 获取习惯记录列表
   * @param date - 可选特定日期
   * @param startDate - 可选开始日期
   * @param endDate - 可选结束日期
   */
  async getSessions(date?: string, startDate?: string, endDate?: string): Promise<Session[]> {
    const HS = await this._b('HabitService');
    return HS.ListSessions(date ?? "", startDate ?? "", endDate ?? "");
  }

  /**
   * 获取习惯详情（含今日进度）
   * @param habitId - 习惯 ID
   * @param date - 可选日期
   */
  async getHabitDetail(habitId: number, date?: string): Promise<HabitDetail> {
    const HS = await this._b('HabitService');
    return HS.GetHabitDetail(habitId, date ?? "");
  }

  /**
   * 创建备份
   */
  async createBackup(): Promise<BackupCreateResult> {
    const BS = await this._b('BackupService');
    return BS.CreateBackup();
  }

  /**
   * 列出所有备份
   */
  async listBackups(): Promise<BackupListResult> {
    const BS = await this._b('BackupService');
    return BS.ListBackups();
  }

  /**
   * 从备份恢复
   * @param name - 备份文件名
   */
  async restoreBackup(name: string): Promise<BackupRestoreResult> {
    const BS = await this._b('BackupService');
    return BS.RestoreBackup(name);
  }

  /**
   * 删除备份
   * @param name - 备份文件名
   */
  async deleteBackup(name: string): Promise<BackupVerifyResult> {
    const BS = await this._b('BackupService');
    return BS.DeleteBackup(name);
  }

  /**
   * 验证备份有效性
   */
  async verifyBackup(): Promise<BackupVerifyResult> {
    const BS = await this._b('BackupService');
    return BS.VerifyBackup();
  }

  /**
   * 获取主密码状态（是否设置、是否解锁、锁定剩余时间）
   */
  async getMasterPasswordStatus(): Promise<{ has_password: boolean; unlocked: boolean; locked_until: number; unlock_time: number }> {
    const BS = await this._b('BackupService');
    return BS.GetMasterPasswordStatus();
  }

  /**
   * 设置主密码
   * @param password - 密码
   */
  async setMasterPassword(password: string): Promise<{ success: boolean; error?: string }> {
    const BS = await this._b('BackupService');
    return BS.SetMasterPassword(password);
  }

  /**
   * 解锁凭证（使用主密码）
   * @param password - 主密码
   */
  async unlockCredentials(password: string): Promise<{ success: boolean; locked_until: number; error?: string }> {
    const BS = await this._b('BackupService');
    return BS.UnlockCredentials(password);
  }

  /**
   * 锁定凭证
   */
  async lockCredentials(): Promise<{ success: boolean }> {
    const BS = await this._b('BackupService');
    return BS.LockCredentials();
  }

  /**
   * 获取备份配置
   */
  async getBackupConfig(): Promise<BackupConfig> {
    const BS = await this._b('BackupService');
    return BS.GetBackupConfig();
  }

  /**
   * 更新备份配置
   * @param config - 备份配置对象
   */
  async updateBackupConfig(config: BackupConfig): Promise<{ success: boolean; error?: string }> {
    const BS = await this._b('BackupService');
    return BS.UpdateBackupConfig(JSON.stringify(config));
  }

  /**
   * 上传壁纸（Android 不可用，返回空）
   * @param _file - 壁纸文件
   */
  async uploadWallpaper(_file: File): Promise<WallpaperUploadResult> {
    // Wallpaper upload is not available on Android (no HTTP server)
    // Return a dummy response so the UI doesn't break
    return { filename: "" };
  }

  /**
   * 列出已上传壁纸（Android 不可用，返回空）
   */
  async listWallpapers(): Promise<WallpaperListResult[]> {
    return [];
  }

  /**
   * 删除壁纸（Android 不可用）
   * @param _filename - 壁纸文件名
   */
  async deleteWallpaper(_filename: string): Promise<WallpaperDeleteResult> {
    return { success: true };
  }

  /**
   * 从 URL 获取壁纸（Android 不可用，返回空）
   * @param _url - 壁纸 URL
   */
  async fetchWallpaperByUrl(_url: string): Promise<WallpaperUploadResult> {
    return { filename: "" };
  }
}

// Re-export the type guard
export { isAndroid };
