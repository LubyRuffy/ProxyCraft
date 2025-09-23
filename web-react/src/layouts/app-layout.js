import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { Outlet, NavLink, useNavigate } from 'react-router-dom';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { Settings } from 'lucide-react';
const navItems = [
    { label: '流量列表', to: '/traffic' },
];
export function AppLayout() {
    const navigate = useNavigate();
    const handleSettingsClick = () => {
        navigate('/settings');
    };
    return (_jsxs("div", { className: "flex min-h-screen flex-col bg-background text-foreground", children: [_jsx("header", { className: "border-b bg-card/70 backdrop-blur", children: _jsxs("div", { className: "mx-auto flex h-14 w-full items-center justify-between px-6", children: [_jsxs("div", { className: "flex items-center gap-6", children: [_jsx("span", { className: "text-sm font-semibold tracking-tight", children: "ProxyCraft Console" }), _jsx("nav", { className: "flex items-center gap-3 text-sm text-muted-foreground", children: navItems.map((item) => (_jsx(NavLink, { to: item.to, className: ({ isActive }) => cn('rounded-md px-2 py-1 transition-colors hover:text-foreground', isActive && 'bg-secondary text-secondary-foreground'), end: true, children: item.label }, item.to))) })] }), _jsxs(Button, { variant: "outline", size: "sm", onClick: handleSettingsClick, children: [_jsx(Settings, { className: "h-4 w-4 mr-2" }), "\u8BBE\u7F6E"] })] }) }), _jsx("main", { className: "flex-1", children: _jsx(Outlet, {}) })] }));
}
