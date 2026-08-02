import type { FunctionalComponent, VNode } from "preact";
import { useEffect, useState, useRef, useCallback } from "preact/hooks";
import { Header } from "./components/Header";
import { TabPanel } from "./components/TabPanel";
import { BasicSettings } from "./components/BasicSettings";
import { WallpaperModal } from "./components/WallpaperModal";
import { CountdownSettings } from "./components/CountdownSettings";
import { StopwatchSettings } from "./components/StopwatchSettings";
import { BackupTab } from "./components/settings/BackupTab";
import { MasterPasswordModal } from "./components/MasterPasswordModal";
import { t, setLanguage } from "./utils/i18n";
import { getAPIClient } from "./utils/apiClientSingleton";
import type { BackupConfig } from "./types/api";
import { isPerfDebugEnabled, isWebViewRuntime, logPerf } from "./utils/logger";
import { ClockIconComponent, CheckIconComponent, ResetIcon, SettingsIcon, BackupIcon } from "./utils/icons";
import { loadAudioPreferences, normalizeAudioPreferences, saveAudioPreferences, DEFAULT_AUDIO_PREFERENCES } from "./utils/audio";
import { STORAGE_KEYS } from "./utils/constants";
import { applyTheme, applyLightStyle } from "./hooks/useAppSettings";

interface SettingsPageProps {
  onBackClick?: () => void;
  wallpaper?: string;
  onWallpaperChange?: (wallpaper: string) => void;
}

const TABS: { id: string; labelKey: string; icon?: VNode }[] = [
  { id: "basic", labelKey: "settings.tabs.basic", icon: <SettingsIcon /> },
  { id: "countdown", labelKey: "settings.tabs.countdown", icon: <ClockIconComponent /> },
  { id: "stopwatch", labelKey: "settings.tabs.stopwatch", icon: <ClockIconComponent /> },
  { id: "backup", labelKey: "settings.tabs.backup", icon: <BackupIcon /> },
];

interface BasicSettingsConfig {
  timezone: number;
  language: string;
  default_mode: string;
  theme_mode: string;
  wallpaper: string;
  sound_enabled: boolean;
  sound_tick: boolean;
  sound_finish: boolean;
  sound_volume: number;
  layout_density: string;
  time_display_style: string;
  light_style: string;
}

interface ClockDefault {
  duration_seconds: number;
  loop: boolean;
  loop_count: number;
  loop_interval_seconds: number;
}

interface StopwatchDefault {
  max_seconds: number;
}

interface ClockDefaults {
  countdown: ClockDefault;
  stopwatch: StopwatchDefault;
}

interface SettingsConfig {
  basic: BasicSettingsConfig;
  clock_defaults: ClockDefaults;
  backup_enabled?: boolean;
  backup_target_type?: string;
  backup_local_path?: string;
  backup_webdav_url?: string;
  backup_webdav_username?: string;
  backup_webdav_password?: string;
  backup_webdav_path_prefix?: string;
  backup_s3_endpoint?: string;
  backup_s3_bucket?: string;
  backup_s3_region?: string;
  backup_s3_access_key?: string;
  backup_s3_secret_key?: string;
  backup_s3_path_prefix?: string;
  backup_auto_interval_hours?: number;
  backup_max_backups?: number;
}

const DEFAULT_CONFIG: SettingsConfig = {
  basic: {
    timezone: 8,
    language: "ZH",
    default_mode: "countdown",
    theme_mode: "dark",
    wallpaper: "",
    sound_enabled: true,
    sound_tick: true,
    sound_finish: true,
    sound_volume: 80,
    layout_density: "normal",
    time_display_style: "classic",
    light_style: "paper",
  },
  clock_defaults: {
    countdown: {
      duration_seconds: 1500,
      loop: false,
      loop_count: 0,
      loop_interval_seconds: 0,
    },
    stopwatch: {
      max_seconds: 86400,
    },
  },
};

export const SettingsPage: FunctionalComponent<SettingsPageProps> = ({
  onBackClick,
  wallpaper,
  onWallpaperChange,
}) => {
  const apiClientRef = useRef<ReturnType<typeof getAPIClient> | null>(null);
  const [config, setConfig] = useState<SettingsConfig>(DEFAULT_CONFIG);
  const [activeTab, setActiveTab] = useState("basic");
  const [isSaving, setIsSaving] = useState(false);
  const [saveMessage, setSaveMessage] = useState("");
  const [showWallpaperModal, setShowWallpaperModal] = useState(false);
  const [masterPasswordModalOpen, setMasterPasswordModalOpen] = useState(false);
  const [masterPasswordModalMode, setMasterPasswordModalMode] = useState<"setup" | "unlock">("setup");
  const animationsEnabled = !isWebViewRuntime();
  const pendingInteractionLabelRef = useRef<string | null>(null);
  const pendingInteractionStartRef = useRef<number>(0);

  const markInteraction = useCallback((label: string) => {
    if (!isPerfDebugEnabled()) return;
    pendingInteractionLabelRef.current = label;
    pendingInteractionStartRef.current = performance.now();
  }, []);

  const handleTabChange = useCallback((tabId: string) => {
    markInteraction(`tab.change:${tabId}`);
    setActiveTab(tabId);
  }, [markInteraction]);

  useEffect(() => {
    apiClientRef.current = getAPIClient();
  }, []);

  useEffect(() => {
    const modeTab = config.basic.default_mode === "countdown" ? "countdown" : "stopwatch";
    setActiveTab(modeTab);
  }, [config.basic.default_mode]);

  const loadSettings = async () => {
    const startAt = performance.now();
    if (!apiClientRef.current) {
      setSaveMessage(t("errors.offline.message"));
      return;
    }

    try {
      const apiStartAt = performance.now();
      const settings = await apiClientRef.current.getSettings();
      const apiDurationMs = Math.round(performance.now() - apiStartAt);

      const normalizeStartAt = performance.now();
      const s = settings as any;
      const localAudioPreferences = loadAudioPreferences();
      const audioPreferences = normalizeAudioPreferences({
        sound_enabled: s?.basic?.sound_enabled ?? localAudioPreferences.sound_enabled,
        sound_tick: s?.basic?.sound_tick ?? localAudioPreferences.sound_tick,
        sound_finish: s?.basic?.sound_finish ?? localAudioPreferences.sound_finish,
        sound_volume: s?.basic?.sound_volume ?? localAudioPreferences.sound_volume,
      });
      saveAudioPreferences(audioPreferences);
      
      const localLayoutDensity = localStorage.getItem(STORAGE_KEYS.LAYOUT_DENSITY) || "normal";
      const localTimeDisplayStyle = localStorage.getItem(STORAGE_KEYS.TIME_DISPLAY_STYLE) || "classic";
      const localLightStyle = localStorage.getItem(STORAGE_KEYS.LIGHT_STYLE) || "paper";
      
      const loadedConfig: SettingsConfig = {
        basic: {
          timezone: s?.basic?.timezone ?? 8,
          language: s?.basic?.language ?? "ZH",
          default_mode: s?.basic?.default_mode ?? "countdown",
          theme_mode: s?.basic?.theme_mode ?? "dark",
          wallpaper: wallpaper || s?.basic?.wallpaper || "",
          sound_enabled: audioPreferences.sound_enabled,
          sound_tick: audioPreferences.sound_tick,
          sound_finish: audioPreferences.sound_finish,
          sound_volume: audioPreferences.sound_volume,
          layout_density: localLayoutDensity,
          time_display_style: localTimeDisplayStyle,
          light_style: localLightStyle,
        },
        clock_defaults: {
          countdown: s?.clock_defaults?.countdown ? {
            duration_seconds: s.clock_defaults.countdown?.duration_seconds ?? 1500,
            loop: s.clock_defaults.countdown?.loop ?? false,
            loop_count: s.clock_defaults.countdown?.loop_count ?? 0,
            loop_interval_seconds: s.clock_defaults.countdown?.loop_interval_seconds ?? 0,
          } : DEFAULT_CONFIG.clock_defaults.countdown,
          stopwatch: s?.clock_defaults?.stopwatch ? {
            max_seconds: s.clock_defaults.stopwatch?.max_seconds ?? 86400,
          } : DEFAULT_CONFIG.clock_defaults.stopwatch,
        },
      };
      const normalizeDurationMs = Math.round(performance.now() - normalizeStartAt);
      
      const setStateStartAt = performance.now();
      setConfig(loadedConfig);

      const backupResult = await apiClientRef.current.getBackupConfig().catch(() => null);
      if (backupResult) {
        const bc = backupResult as any;
        setConfig((prev) => ({
          ...prev,
          backup_enabled: bc.enabled ?? false,
          backup_target_type: bc.target_type ?? 'local',
          backup_local_path: bc.local_path ?? '',
          backup_webdav_url: bc.webdav_url ?? '',
          backup_webdav_username: bc.webdav_username ?? '',
          backup_webdav_password: bc.webdav_password ?? '',
          backup_webdav_path_prefix: bc.webdav_path_prefix ?? 'little_timer/',
          backup_s3_endpoint: bc.s3_endpoint ?? '',
          backup_s3_bucket: bc.s3_bucket ?? '',
          backup_s3_region: bc.s3_region ?? '',
          backup_s3_access_key: bc.s3_access_key ?? '',
          backup_s3_secret_key: bc.s3_secret_key ?? '',
          backup_s3_path_prefix: bc.s3_path_prefix ?? '',
        }));
      }

      logPerf("Settings.load.success", {
        durationMs: Math.round(performance.now() - startAt),
        apiDurationMs,
        normalizeDurationMs,
        setStateScheduleMs: Math.round(performance.now() - setStateStartAt),
        defaultMode: loadedConfig.basic.default_mode,
        language: loadedConfig.basic.language,
      });
    } catch (error) {
      console.error("加载设置失败:", error);
      setSaveMessage(t("errors.offline.message"));
      logPerf("Settings.load.error", {
        durationMs: Math.round(performance.now() - startAt),
      });
    }
  };

  useEffect(() => {
    void loadSettings();
  }, []);

  useEffect(() => {
    const startAt = performance.now();
    const themeMode: string = config.basic.theme_mode != null ? String(config.basic.theme_mode) : "dark";
    applyTheme(themeMode);
    logPerf("Settings.theme.applied", {
      themeMode,
      durationMs: Math.round(performance.now() - startAt),
    });
  }, [config.basic.theme_mode]);

  useEffect(() => {
    const startAt = performance.now();
    const lightStyle: string = config.basic.light_style != null ? String(config.basic.light_style) : "paper";
    applyLightStyle(lightStyle);
    logPerf("Settings.lightStyle.applied", {
      lightStyle,
      durationMs: Math.round(performance.now() - startAt),
    });
  }, [config.basic.light_style]);

  useEffect(() => {
    const startAt = performance.now();
    const lang = String(config.basic.language ?? "ZH");
    setLanguage(lang);
    logPerf("Settings.language.changed", {
      language: lang,
      scheduleMs: Math.round(performance.now() - startAt),
    });
  }, [config.basic.language]);

  useEffect(() => {
    const wallpaper = String(config.basic.wallpaper ?? "");
    onWallpaperChange?.(wallpaper);
  }, [config.basic.wallpaper, onWallpaperChange]);

  const handleSave = () => {
    const startAt = performance.now();
    setIsSaving(true);
    setSaveMessage("");

    const audioPreferences = normalizeAudioPreferences({
      sound_enabled: config.basic.sound_enabled,
      sound_tick: config.basic.sound_tick,
      sound_finish: config.basic.sound_finish,
      sound_volume: config.basic.sound_volume,
    });
    saveAudioPreferences(audioPreferences);
    
    // 保存布局密度到localStorage
    localStorage.setItem(STORAGE_KEYS.LAYOUT_DENSITY, String(config.basic.layout_density ?? "normal"));
    window.dispatchEvent(new CustomEvent("setting-change", {
      detail: { key: "layout_density", value: String(config.basic.layout_density ?? "normal") }
    }));
    localStorage.setItem(STORAGE_KEYS.TIME_DISPLAY_STYLE, String(config.basic.time_display_style ?? "classic"));
    window.dispatchEvent(new CustomEvent("setting-change", {
      detail: { key: "time_display_style", value: String(config.basic.time_display_style ?? "classic") }
    }));
    localStorage.setItem(STORAGE_KEYS.LIGHT_STYLE, String(config.basic.light_style ?? "paper"));
    window.dispatchEvent(new CustomEvent("setting-change", {
      detail: { key: "light_style", value: String(config.basic.light_style ?? "paper") }
    }));

    const {
      sound_enabled: _sound_enabled,
      sound_tick: _sound_tick,
      sound_finish: _sound_finish,
      sound_volume: _sound_volume,
      layout_density: _layout_density,
      time_display_style: _time_display_style,
      light_style: _light_style,
      ...serverBasic
    } =
      config.basic;
    const serverConfig = {
      ...config,
      basic: serverBasic,
    };

    if (apiClientRef.current) {
      void apiClientRef.current.updateSettings(serverConfig)
        .then(() => {
          setSaveMessage(t("common.save_success"));
          setTimeout(() => setSaveMessage(""), 3000);
          logPerf("Settings.save.success", {
            durationMs: Math.round(performance.now() - startAt),
          });
        })
         .catch((error: unknown) => {
          const errorMessage = error instanceof Error ? error.message : "未知错误";
          setSaveMessage(t("validation.save_error", { error: errorMessage }));
          logPerf("Settings.save.error", {
            durationMs: Math.round(performance.now() - startAt),
            error: errorMessage,
          });
        })
        .finally(() => {
          setIsSaving(false);
        });

      const backupConfig: BackupConfig = {
        enabled: config.backup_enabled ?? false,
        target_type: (config.backup_target_type ?? 'local') as 'local' | 'webdav' | 's3',
        local_path: config.backup_local_path ?? '',
        webdav_url: config.backup_webdav_url ?? '',
        webdav_username: config.backup_webdav_username ?? '',
        webdav_password: config.backup_webdav_password ?? '',
        webdav_path_prefix: config.backup_webdav_path_prefix ?? 'little_timer/',
        s3_endpoint: config.backup_s3_endpoint ?? '',
        s3_bucket: config.backup_s3_bucket ?? '',
        s3_region: config.backup_s3_region ?? '',
        s3_access_key: config.backup_s3_access_key ?? '',
        s3_secret_key: config.backup_s3_secret_key ?? '',
        s3_path_prefix: config.backup_s3_path_prefix ?? '',
      };
      void apiClientRef.current.updateBackupConfig(backupConfig)
        .catch((err: unknown) => {
          const errObj = err as { action?: { type: string; target: string; params?: { mode: string } } };
          if (errObj.action && errObj.action.type === "show_modal" && errObj.action.target === "master_password") {
            const mode = errObj.action.params?.mode as "setup" | "unlock" || "setup";
            setMasterPasswordModalMode(mode);
            setMasterPasswordModalOpen(true);
          } else {
            console.error('Failed to save backup config:', String(err));
          }
        });
    }
  };

  const handleReset = () => {
    if (confirm(t("common.reset_confirm"))) {
      const startAt = performance.now();
      setConfig(DEFAULT_CONFIG);
      saveAudioPreferences(DEFAULT_AUDIO_PREFERENCES);
      localStorage.setItem(STORAGE_KEYS.LAYOUT_DENSITY, "normal");
      localStorage.setItem(STORAGE_KEYS.TIME_DISPLAY_STYLE, "classic");
      localStorage.setItem(STORAGE_KEYS.LIGHT_STYLE, "paper");
      setSaveMessage(t("common.save_hint"));
      logPerf("Settings.reset", {
        durationMs: Math.round(performance.now() - startAt),
      });
    }
  };

  useEffect(() => {
    if (!isPerfDebugEnabled()) return;
    logPerf("Settings.tab.changed", {
      activeTab,
      defaultMode: config.basic.default_mode,
    });
  }, [activeTab, config.basic.default_mode]);

  useEffect(() => {
    if (!isPerfDebugEnabled()) return;
    if (!pendingInteractionLabelRef.current) return;

    const interactionLabel = pendingInteractionLabelRef.current;
    const interactionStartAt = pendingInteractionStartRef.current;
    pendingInteractionLabelRef.current = null;

    requestAnimationFrame(() => {
      logPerf("Settings.interaction.frame", {
        label: interactionLabel,
        durationMs: Math.round(performance.now() - interactionStartAt),
        activeTab,
      });
    });
  }, [activeTab, config]);

  useEffect(() => {
    if (!isPerfDebugEnabled()) return;
    if (typeof PerformanceObserver === "undefined") return;

    const observer = new PerformanceObserver((list) => {
      const entries = list.getEntries();
      for (const entry of entries) {
        logPerf("Settings.longtask", {
          name: entry.name,
          durationMs: Math.round(entry.duration),
          activeTab,
        });
      }
    });

    try {
      observer.observe({ type: "longtask", buffered: true });
    } catch {
      // 部分 WebView 不支持 longtask，忽略即可。
    }

    return () => {
      observer.disconnect();
    };
  }, [activeTab]);

  return (
    <div
      className={`flex flex-col flex-1 text-base-content transition-colors duration-300 overflow-hidden bg-transparent ${
        animationsEnabled ? "animate-fadeIn" : ""
      }`}
    >
      <div
        className="flex flex-col w-full h-full"
        style={(config.basic.wallpaper || wallpaper) ? { backgroundColor: "rgba(0,0,0,0.15)" } : {}}
      >
        <Header
          title={t("common.settings_title")}
          showBack={true}
          onBackClick={onBackClick}
          showSettings={false}
        />

        <TabPanel
          tabs={TABS.map((tab) => ({ ...tab, label: t(tab.labelKey) }))}
          activeTab={activeTab}
          onTabChange={handleTabChange}
          isAnimated={animationsEnabled}
        >
          {activeTab === "basic" && (
            <BasicSettings
              config={config.basic}
              isAnimated={animationsEnabled}
              onWallpaperClick={() => setShowWallpaperModal(true)}
              onChange={(newBasic) => {
                markInteraction("basic.config.change");
                setConfig((prev) => {
                  const next = { ...prev, basic: newBasic };

                  if (newBasic.default_mode !== "countdown") {
                    next.clock_defaults = {
                      ...prev.clock_defaults,
                      countdown: {
                        ...prev.clock_defaults.countdown,
                        loop: false,
                        loop_count: 0,
                        loop_interval_seconds: 0,
                      },
                    };
                  }

                  return next;
                });
              }}
            />
          )}

          {activeTab === "countdown" && (
            <CountdownSettings
              config={config.clock_defaults.countdown}
              showLoopControls={config.basic.default_mode === "countdown"}
              isAnimated={animationsEnabled}
              onChange={(newCountdown) => {
                markInteraction("countdown.config.change");
                setConfig({
                  ...config,
                  clock_defaults: {
                    ...config.clock_defaults,
                    countdown: newCountdown,
                  },
                });
              }}
            />
          )}

          {activeTab === "stopwatch" && (
            <StopwatchSettings
              config={config.clock_defaults.stopwatch}
              isAnimated={animationsEnabled}
              onChange={(newStopwatch) => {
                markInteraction("stopwatch.config.change");
                setConfig({
                  ...config,
                  clock_defaults: {
                    ...config.clock_defaults,
                    stopwatch: newStopwatch,
                  },
                });
              }}
            />
          )}
          {activeTab === "backup" && (
            <BackupTab
              config={config}
              onChange={(newConfig) => {
                setConfig(newConfig);
              }}
            />
          )}
        </TabPanel>

        <div
          className={`my-surface-panel flex gap-2 sm:gap-3 md:gap-4 items-center justify-center px-4 sm:px-6 md:px-8 py-4 sm:py-6 md:py-8 flex-wrap flex-shrink-0 ${
            animationsEnabled ? "animate-slideUp" : ""
          }`}
          style={animationsEnabled ? { animationDelay: "0.3s", animationFillMode: "both" } : undefined}
        >
          <button
            onClick={handleSave}
            disabled={isSaving}
            className="btn-primary inline-flex items-center gap-2"
          >
            {!isSaving && <CheckIconComponent />}
            {isSaving ? t("common.saving") : t("common.save")}
          </button>
          <button onClick={handleReset} className="my-btn-secondary inline-flex items-center gap-2">
            <ResetIcon />
            {t("common.reset_default")}
          </button>
          {saveMessage && (
            <div className="px-4 sm:px-5 py-2 rounded-lg text-xs sm:text-sm font-medium bg-secondary-dark text-accent-dark animate-pulse">
              {saveMessage}
            </div>
          )}
        </div>
      </div>

      <WallpaperModal
        isOpen={showWallpaperModal}
        value={config.basic.wallpaper || ""}
        onClose={() => setShowWallpaperModal(false)}
        onChange={(wallpaper) => {
          setConfig((prev) => ({
            ...prev,
            basic: { ...prev.basic, wallpaper },
          }));
          onWallpaperChange?.(wallpaper);
        }}
      />

      <MasterPasswordModal
        isOpen={masterPasswordModalOpen}
        mode={masterPasswordModalMode}
        onSuccess={() => {
          setMasterPasswordModalOpen(false);
          setSaveMessage(t("master_password.unlock_success"));
          setTimeout(() => setSaveMessage(""), 3000);
        }}
        onClose={() => setMasterPasswordModalOpen(false)}
      />
    </div>
  );
};
