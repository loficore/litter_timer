// API 响应类型定义

/**
 * @description 定时器当前运行状态
 */
export interface TimerState {
  /** 当前计时值（秒） */
  time: number;
  /** 已经过的时间（秒） */
  elapsed?: number;
  /** 计时模式 */
  mode: "countdown" | "stopwatch";
  /** 是否正在运行 */
  is_running: boolean;
  /** 是否已结束 */
  is_finished: boolean;
  /** 是否处于休息阶段 */
  in_rest: boolean;
  /** 剩余循环次数 */
  loop_remaining: number;
  /** 循环总次数 */
  loop_total: number;
  /** 剩余休息时间（秒） */
  rest_remaining: number;
  /** 时区偏移量 */
  timezone: number;
  /** 关联的习惯 ID */
  habit_id?: number;
}

/**
 * @description 应用基础设置
 */
export interface BasicSettings {
  /** 时区偏移量 */
  timezone: number;
  /** 界面语言 */
  language: string;
  /** 默认计时模式 */
  default_mode: string;
  /** 主题模式 */
  theme_mode: string;
  /** 壁纸路径或地址 */
  wallpaper?: string;
  /** 是否启用声音 */
  sound_enabled?: boolean;
  /** 是否启用滴答声 */
  sound_tick?: boolean;
  /** 是否启用结束提示音 */
  sound_finish?: boolean;
  /** 声音音量 */
  sound_volume?: number;
}

/**
 * @description 倒计时默认设置
 */
export interface CountdownDefaults {
  /** 默认持续时间（秒） */
  duration_seconds: number;
  /** 是否循环计时 */
  loop: boolean;
  /** 默认循环次数 */
  loop_count: number;
  /** 循环间隔时间（秒） */
  loop_interval_seconds: number;
}

/**
 * @description 正计时默认设置
 */
export interface StopwatchDefaults {
  /** 最大计时时间（秒） */
  max_seconds: number;
}

/**
 * @description 应用完整设置
 */
export interface Settings {
  /** 基础设置 */
  basic: BasicSettings;
  /** 倒计时设置 */
  countdown?: CountdownDefaults;
  /** 正计时设置 */
  stopwatch?: StopwatchDefaults;
  /** 世界时钟设置 */
  world_clock?: object;
}

/**
 * @description 习惯集合信息
 */
export interface HabitSet {
  /** 集合 ID */
  id: number;
  /** 集合名称 */
  name: string;
  /** 集合描述 */
  description: string;
  /** 集合颜色 */
  color: string;
  /** 壁纸路径或地址 */
  wallpaper?: string;
  /** 创建时间 */
  created_at?: string;
}

/**
 * @description 习惯信息
 */
export interface Habit {
  /** 习惯 ID */
  id: number;
  /** 所属集合 ID */
  set_id: number;
  /** 习惯名称 */
  name: string;
  /** 目标时长（秒） */
  goal_seconds: number;
  /** 习惯颜色 */
  color: string;
  /** 壁纸路径或地址 */
  wallpaper?: string;
  /** 创建时间 */
  created_at?: string;
}

/**
 * @description 一次习惯计时会话
 */
export interface Session {
  /** 会话 ID */
  id: number;
  /** 关联的习惯 ID */
  habit_id: number;
  /** 会话持续时间（秒） */
  duration_seconds: number;
  /** 完成次数 */
  count: number;
  /** 开始时间 */
  started_at: string;
  /** 会话日期 */
  date: string;
}

/**
 * @description 习惯详情及进度信息
 */
export interface HabitDetail {
  /** 习惯 ID */
  id: number;
  /** 习惯名称 */
  name: string;
  /** 目标时长（秒） */
  goal_seconds: number;
  /** 习惯颜色 */
  color: string;
  /** 今日累计时长（秒） */
  today_seconds: number;
  /** 当前进度百分比 */
  progress_percent: number;
}

/**
 * @description 定时器实时进度
 */
export interface TimerProgress {
  /** 当前会话 ID */
  session_id: number | null;
  /** 当前习惯 ID */
  habit_id: number | null;
  /** 计时模式 */
  mode: string;
  /** 是否正在运行 */
  is_running: boolean;
  /** 是否已暂停 */
  is_paused: boolean;
  /** 是否已结束 */
  is_finished: boolean;
  /** 已经过的时间（秒） */
  elapsed_seconds: number;
  /** 剩余时间（秒） */
  remaining_seconds: number;
  /** 是否处于休息阶段 */
  in_rest: boolean;
}

/**
 * @description 开始计时结果
 */
export interface TimerStartResult {
  /** 关联的习惯 ID */
  habit_id: number | null;
  /** 创建的会话 ID */
  session_id?: number;
}

/**
 * @description 完成计时结果
 */
export interface TimerFinishResult {
  /** 操作状态 */
  status: string;
  /** 已经过的时间（秒） */
  elapsed_seconds: number;
  /** 关联的会话 ID */
  session_id?: number;
}

/**
 * @description 休息阶段结果
 */
export interface RestResult {
  /** 休息时长（秒） */
  rest_seconds: number;
}

/**
 * @description 恢复计时结果
 */
export interface ResumeResult {
  /** 关联的习惯 ID */
  habit_id: number | null;
}

/**
 * @description 创建会话结果
 */
export interface CreateSessionResult {
  /** 创建的会话 ID */
  id?: number;
  /** 关联的习惯 ID */
  habit_id: number;
  /** 会话持续时间（秒） */
  duration_seconds: number;
  /** 完成次数 */
  count: number;
  /** 会话日期 */
  date: string;
}

/**
 * @description 数据备份配置
 */
export interface BackupConfig {
  /** 是否启用备份 */
  enabled: boolean;
  /** 备份目标类型 */
  target_type: "local" | "webdav" | "s3";
  /** 本地备份路径 */
  local_path?: string;
  /** WebDAV 地址 */
  webdav_url?: string;
  /** WebDAV 用户名 */
  webdav_username?: string;
  /** WebDAV 密码 */
  webdav_password?: string;
  webdav_path_prefix?: string;
  /** S3 服务端点 */
  s3_endpoint?: string;
  /** S3 存储桶名称 */
  s3_bucket?: string;
  /** S3 区域 */
  s3_region?: string;
  /** S3 访问密钥 */
  s3_access_key?: string;
  /** S3 私密密钥 */
  s3_secret_key?: string;
  /** S3 路径前缀 */
  s3_path_prefix?: string;
  /** 自动备份间隔（小时） */
  auto_interval_hours?: number;
  /** 最大备份数量 */
  max_backups?: number;
}

/**
 * @description 备份列表项
 */
export interface BackupListItem {
  /** 备份名称 */
  name: string;
  /** 备份时间戳 */
  timestamp: number;
  /** 备份大小（字节） */
  size_bytes: number;
}

/**
 * @description 备份列表结果
 */
export interface BackupListResult {
  /** 请求是否成功 */
  success: boolean;
  /** 备份列表 */
  backups: BackupListItem[];
  /** 错误信息 */
  error?: string;
}

/**
 * @description 创建备份结果
 */
export interface BackupCreateResult {
  /** 请求是否成功 */
  success: boolean;
  /** 备份文件路径 */
  backup_path?: string;
  /** 错误信息 */
  error?: string;
}

/**
 * @description 恢复备份结果
 */
export interface BackupRestoreResult {
  /** 请求是否成功 */
  success: boolean;
  /** 错误信息 */
  error?: string;
}

/**
 * @description 校验备份结果
 */
export interface BackupVerifyResult {
  /** 请求是否成功 */
  success: boolean;
  /** 错误信息 */
  error?: string;
}

/**
 * @description 壁纸上传结果
 */
export interface WallpaperUploadResult {
  /** 上传后的文件名 */
  filename: string;
}

/**
 * @description 壁纸列表结果
 */
export interface WallpaperListResult {
  /** 壁纸名称 */
  name: string;
  /** 文件大小（字节） */
  size: number;
  /** 被 habits / habit_sets / settings 引用的次数 */
  refs: number;
}

/**
 * @description 壁纸删除结果
 */
export interface WallpaperDeleteResult {
  /** 删除是否成功 */
  success: boolean;
}

/**
 * @description 开始计时选项
 */
export interface TimerStartOptions {
  /** 计时模式 */
  mode?: "countdown" | "stopwatch";
  /** 工作时长（秒） */
  workDuration?: number;
  /** 休息时长（秒） */
  restDuration?: number;
  /** 循环次数 */
  loopCount?: number;
}
