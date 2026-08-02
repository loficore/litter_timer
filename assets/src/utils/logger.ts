// 日志辅助函数 - 支持分类和后端保存

export type LogCategory = 'lifecycle' | 'operation' | 'network' | 'error' | 'perf';
export type FrontendLogLevel = 'error' | 'info' | 'debug';
export type FrontendRuntime = 'webview' | 'browser' | 'unknown';

const PERF_DEBUG_QUERY_KEY = 'debugPerf';
const PERF_DEBUG_STORAGE_KEY = 'lt_debug_perf';
const LOG_LEVEL_QUERY_KEY = 'logLevel';
const LOG_LEVEL_STORAGE_KEY = 'lt_log_level';
const RUNTIME_QUERY_KEY = 'runtime';

let perfDebugCached: boolean | null = null;
let frontendLogLevelCached: FrontendLogLevel | null = null;

const normalizeRuntime = (value: string | null): FrontendRuntime | null => {
  if (!value) return null;
  const normalized = value.trim().toLowerCase();
  if (normalized === 'webview' || normalized === 'browser') {
    return normalized;
  }
  return null;
};

export const getFrontendRuntime = (): FrontendRuntime => {
  if (typeof window === 'undefined') return 'unknown';

  try {
    const search = new URLSearchParams(window.location.search);
    const fromQuery = normalizeRuntime(search.get(RUNTIME_QUERY_KEY));
    if (fromQuery) return fromQuery;
  } catch {
    // 忽略 URL 解析异常
  }

  if (window.webui) return 'webview';
  return 'browser';
};

export const isWebViewRuntime = (): boolean => getFrontendRuntime() === 'webview';

const readPerfDebugFlag = (): boolean => {
  if (typeof window === 'undefined') return false;

  try {
    const search = new URLSearchParams(window.location.search);
    if (search.has(PERF_DEBUG_QUERY_KEY)) {
      const value = search.get(PERF_DEBUG_QUERY_KEY);
      if (value === '0' || value === 'false') return false;
      return true;
    }
  } catch {
    // 忽略 URL 解析异常
  }

  try {
    const value = localStorage.getItem(PERF_DEBUG_STORAGE_KEY);
    if (value === '1') return true;
    if (value === '0') return false;
  } catch {
    // 忽略 localStorage 异常
  }

  // WebView 调试优先开启，便于定位页面卡顿。
  return isWebViewRuntime();
};

export const isPerfDebugEnabled = (): boolean => {
  if (perfDebugCached === null) {
    perfDebugCached = readPerfDebugFlag();
  }
  return perfDebugCached;
};

export const setPerfDebugEnabled = (enabled: boolean) => {
  perfDebugCached = enabled;
  if (typeof window === 'undefined') return;
  try {
    if (enabled) {
      localStorage.setItem(PERF_DEBUG_STORAGE_KEY, '1');
    } else {
      localStorage.removeItem(PERF_DEBUG_STORAGE_KEY);
    }
  } catch {
    // 忽略 localStorage 不可用场景
  }
};

const normalizeLogLevel = (value: string | null): FrontendLogLevel | null => {
  if (!value) return null;
  const normalized = value.trim().toLowerCase();
  if (normalized === 'error' || normalized === 'info' || normalized === 'debug') {
    return normalized;
  }
  return null;
};

const readFrontendLogLevel = (): FrontendLogLevel => {
  if (typeof window === 'undefined') return 'info';

  try {
    const search = new URLSearchParams(window.location.search);
    const fromQuery = normalizeLogLevel(search.get(LOG_LEVEL_QUERY_KEY));
    if (fromQuery) return fromQuery;
  } catch {
    // 忽略 URL 解析异常
  }

  try {
    const fromStorage = normalizeLogLevel(localStorage.getItem(LOG_LEVEL_STORAGE_KEY));
    if (fromStorage) return fromStorage;
  } catch {
    // 忽略 localStorage 异常
  }

  return 'debug';
};

export const getFrontendLogLevel = (): FrontendLogLevel => {
  if (frontendLogLevelCached === null) {
    frontendLogLevelCached = readFrontendLogLevel();
  }
  return frontendLogLevelCached;
};

export const setFrontendLogLevel = (level: FrontendLogLevel) => {
  frontendLogLevelCached = level;
  if (typeof window === 'undefined') return;

  try {
    localStorage.setItem(LOG_LEVEL_STORAGE_KEY, level);
  } catch {
    // 忽略 localStorage 异常
  }
};

const shouldLog = (category: LogCategory): boolean => {
  if (category === 'error') return true;

  const level = getFrontendLogLevel();
  if (level === 'error') return false;
  if (level === 'info') return category !== 'perf';
  return true;
};

const logToBackend = (category: LogCategory, message: string, level: 'info' | 'error') => {
  if (typeof window === 'undefined') return;

  // Android: route through Wails JS bridge → logcat (tag: WailsJSBridge/JS)
  if ((window as any).wails && typeof (window as any).wails.log === 'function') {
    (window as any).wails.log(level === 'error' ? 'error' : 'info', message);
    return;
  }

  const runtime = getFrontendRuntime();

  // 优先使用 HTTP 日志接口，便于统一落盘。
  fetch('/api/log', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ category, level, message, runtime }),
  }).catch((error) => {
    void error;
  });

  if (window.webui) {
    window.webui.call('log', category, level, message);
  }
};

const formatMessage = (category: LogCategory, msg: string) => {
  const time = new Date().toISOString();
  return `[${category.toUpperCase()}] ${time} - ${msg}`;
};

const getCategoryStyle = (category: LogCategory, isError = false): string => {
  if (isError) return 'color: #dc3545; font-weight: bold;';
  switch (category) {
    case 'lifecycle': return 'color: #667eea; font-weight: bold;';
    case 'operation': return 'color: #28a745; font-weight: bold;';
    case 'network': return 'color: #fd7e14; font-weight: bold;';
    case 'perf': return 'color: #17a2b8; font-weight: bold;';
    case 'error': return 'color: #dc3545; font-weight: bold;';
    default: return 'color: #6c757d; font-weight: bold;';
  }
};

export const logInfo = (msg: string, category: LogCategory = 'operation') => {
  if (!shouldLog(category)) return;
  const formatted = formatMessage(category, msg);
  const runtime = getFrontendRuntime();
  console.log(`%c[前端:${runtime}] ${formatted}`, getCategoryStyle(category));
  logToBackend(category, msg, 'info');
};

export const logSuccess = (msg: string, category: LogCategory = 'operation') => {
  if (!shouldLog(category)) return;
  const formatted = formatMessage(category, msg);
  const runtime = getFrontendRuntime();
  console.log(`%c[前端:${runtime}] ${formatted}`, getCategoryStyle(category));
  logToBackend(category, msg, 'info');
};

export const logError = (msg: string, error?: Error) => {
  const fullMsg = error?.message ? `${msg}: ${error.message}` : msg;
  const stack = error?.stack ? `\n${error.stack}` : '';
  const formatted = formatMessage('error', fullMsg);
  const runtime = getFrontendRuntime();
  console.error(`%c[前端:${runtime}] ${formatted}`, getCategoryStyle('error', true));
  if (stack) {
    console.error(stack);
  }
  logToBackend('error', fullMsg, 'error');
};

export const logPerf = (scope: string, payload?: Record<string, unknown>) => {
  if (!isPerfDebugEnabled() || !shouldLog('perf')) return;

  const detail = payload ? ` ${JSON.stringify(payload)}` : '';
  logInfo(`[PERF] ${scope}${detail}`, 'perf');
};

// 便捷函数 - 按类别日志
export const logLifecycle = (msg: string) => logInfo(msg, 'lifecycle');
export const logOperation = (msg: string) => logInfo(msg, 'operation');
export const logNetwork = (msg: string) => logInfo(msg, 'network');
