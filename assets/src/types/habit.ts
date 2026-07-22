// 习惯追踪相关类型定义

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
 * @description 包含今日进度的习惯信息
 */
export interface HabitWithProgress extends Habit {
  /** 今日累计时长（秒） */
  today_seconds: number;
  /** 今日完成次数 */
  today_count: number;
  /** 当前进度百分比（0-100） */
  progress: number; // 0-100 percentage
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
  /** 连续完成天数 */
  streak: number;
  /** 当前进度百分比 */
  progress_percent: number;
}

/**
 * @description 包含习惯列表的集合信息
 */
export interface HabitSetWithHabits extends HabitSet {
  /** 集合中的习惯列表 */
  habits: HabitWithProgress[];
}

/**
 * @description 每日习惯统计数据
 */
export interface DailyStats {
  /** 统计日期 */
  date: string;
  /** 当日总时长（秒） */
  total_seconds: number;
  /** 当日总完成次数 */
  total_count: number;
  /** 各习惯的当日统计 */
  habits: {
    /** 习惯 ID */
    habit_id: number;
    /** 习惯名称 */
    habit_name: string;
    /** 习惯颜色 */
    habit_color: string;
    /** 习惯累计时长（秒） */
    seconds: number;
    /** 习惯完成次数 */
    count: number;
  }[];
}

/**
 * @description 习惯统计汇总数据
 */
export interface StatsSummary {
  /** 有记录的总天数 */
  total_days: number;
  /** 累计总时长（秒） */
  total_seconds: number;
  /** 累计总会话数 */
  total_sessions: number;
  /** 当前连续完成天数 */
  current_streak: number;
  /** 最长连续完成天数 */
  longest_streak: number;
  /** 日均时长（秒） */
  average_per_day: number;
  /** 目标完成率 */
  completion_rate: number;
}

/**
 * @description 统计查询的日期范围
 */
export interface DateRange {
  /** 开始日期 */
  start: string;
  /** 结束日期 */
  end: string;
}
