/**
 * 侧边栏导航组件
 * 展示应用主要页面（计时、习惯、统计、设置）的入口
 */

import type { FunctionalComponent } from "preact";
import { t } from "../utils/i18n";
import { StarIconComponent, TimerIconComponent, HabitsIconComponent, ChartIcon, SettingsIcon, PhotoIconComponent } from "../utils/icons";

type Page = "timer" | "habits" | "stats" | "settings" | "gallery";

interface SidebarProps {
    /** 当前所在页面 */
    currentPage: Page;
    /** 切换页面回调 */
    onNavigate: (page: Page) => void;
}

const navItems = [
    {
        id: "timer" as const,
        labelKey: "nav.timer",
        icon: (
            <TimerIconComponent className="h-5 w-5" />
        ),
    },
    {
        id: "habits" as const,
        labelKey: "nav.habits",
        icon: (
            <HabitsIconComponent className="h-5 w-5" />
        ),
    },
    {
        id: "stats" as const,
        labelKey: "nav.stats",
        icon: (
            <ChartIcon className="h-5 w-5" />
        ),
    },
    {
        id: "settings" as const,
        labelKey: "nav.settings",
        icon: (
            <SettingsIcon className="h-5 w-5" />
        ),
    },
    {
        id: "gallery" as const,
        labelKey: "nav.gallery",
        icon: (
            <PhotoIconComponent className="h-5 w-5" />
        ),
    },
];

export const Sidebar: FunctionalComponent<SidebarProps> = ({ currentPage, onNavigate }) => {
    return (
        <aside className="my-sidebar flex flex-col w-60 h-full shrink-0">
            {/* Logo */}
            <div className="p-4 shadow-[inset_0_-1px_0_color-mix(in_oklab,var(--my-outline)_22%,transparent)]">
                <h1 className="text-xl font-bold flex items-center gap-2 text-white/90">
                    <StarIconComponent />
                    <span className="text-white">Little Timer</span>
                </h1>
            </div>

            {/* Navigation */}
            <nav className="flex-1 p-2">
                {navItems.map((item) => (
                    <button
                        key={item.id}
                        data-testid={`nav-${item.id}`}
                        className={`my-sidebar-nav-btn ${currentPage === item.id ? "is-active" : ""}`}
                        onClick={() => onNavigate(item.id)}
                    >
                        {item.icon}
                        <span className="font-medium">{t(item.labelKey)}</span>
                    </button>
                ))}
            </nav>

            {/* Footer */}
            <div className="p-4 text-center text-sm text-white/60 shadow-[inset_0_1px_0_color-mix(in_oklab,var(--my-outline)_18%,transparent)]">
                <p>v1.0.0</p>
            </div>
        </aside>
    );
};
