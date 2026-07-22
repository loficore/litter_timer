/**
 * 侧边栏导航组件
 * 展示应用主要页面（计时、习惯、统计、设置）的入口
 */

import type { FunctionalComponent } from "preact";
import { t } from "../utils/i18n";
import { StarIconComponent } from "../utils/icons";

type Page = "timer" | "habits" | "stats" | "settings";

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
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
        ),
    },
    {
        id: "habits" as const,
        labelKey: "nav.habits",
        icon: (
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4" />
            </svg>
        ),
    },
    {
        id: "stats" as const,
        labelKey: "nav.stats",
        icon: (
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
            </svg>
        ),
    },
    {
        id: "settings" as const,
        labelKey: "nav.settings",
        icon: (
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
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
