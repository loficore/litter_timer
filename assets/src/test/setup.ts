// 测试环境设置文件
import { afterEach, vi, beforeAll } from 'vitest'
import { cleanup } from '@testing-library/preact'

// 每个测试后清理
afterEach(() => {
  cleanup()
})

// Mock import.meta
beforeAll(() => {
  vi.stubGlobal('import.meta', {
    env: {},
    glob: vi.fn((pattern, options) => {
      const result: Record<string, unknown> = {};
      if (options?.eager) {
        // 返回空的 i18n 内容，避免加载真实文件
        if (pattern.includes('i18n')) {
          result['../../i18n/zh.toml'] = '';
          result['../../i18n/en.toml'] = '';
          result['../../i18n/jp.toml'] = '';
        }
        return result;
      }
      return () => Promise.resolve(result);
    }),
  });
})

// Mock 图标组件
vi.mock('../utils/icons', async () => {
  const MockIcon = () => null;
  return {
    PlayIconComponent: MockIcon,
    PauseIconComponent: MockIcon,
    ResetIcon: MockIcon,
    SettingsIcon: MockIcon,
    ArrowLeftIconComponent: MockIcon,
    ClockIconComponent: MockIcon,
    TimerIconComponent: MockIcon,
    GlobeIcon: MockIcon,
    StarIconComponent: MockIcon,
    TrashIconComponent: MockIcon,
    PlusIconComponent: MockIcon,
    CheckIconComponent: MockIcon,
    CloseIcon: MockIcon,
    ChevronDownIconComponent: MockIcon,
    ChevronUpIconComponent: MockIcon,
    ChartIcon: MockIcon,
    BackupIcon: MockIcon,
    PlayIcon: MockIcon,
    PauseIcon: MockIcon,
    ArrowPathIcon: MockIcon,
    Cog6ToothIcon: MockIcon,
    ClockIcon: MockIcon,
    GlobeAltIcon: MockIcon,
    StarIcon: MockIcon,
    TrashIcon: MockIcon,
    PlusIcon: MockIcon,
    CheckIcon: MockIcon,
    XMarkIcon: MockIcon,
    ChevronDownIcon: MockIcon,
    ChevronUpIcon: MockIcon,
    ArrowLeftIcon: MockIcon,
    ChartBarIcon: MockIcon,
    ArchiveBoxIcon: MockIcon,
    PencilIconComponent: MockIcon,
    HabitsIconComponent: MockIcon,
    PhotoIconComponent: MockIcon,
    ForwardIconComponent: MockIcon,
  };
});

// Mock window.webui
declare global {
  interface Window {
    webui?: {
      call: (functionName: string, ...args: unknown[]) => void;
    };
  }
}

// Mock localStorage
const localStorageMock = {
  getItem: vi.fn(),
  setItem: vi.fn(),
  removeItem: vi.fn(),
  clear: vi.fn(),
}
global.localStorage = localStorageMock as any

// Mock matchMedia
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
})
