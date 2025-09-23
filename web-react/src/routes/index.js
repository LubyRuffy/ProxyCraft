import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { Routes, Route, Navigate } from 'react-router-dom';
import { AppLayout } from '@/layouts/app-layout';
import { TrafficPage } from '@/pages/traffic';
import { SettingsPage } from '@/pages/settings';
export function AppRoutes() {
    return (_jsx(Routes, { children: _jsxs(Route, { element: _jsx(AppLayout, {}), children: [_jsx(Route, { index: true, element: _jsx(Navigate, { to: "/traffic", replace: true }) }), _jsx(Route, { path: "/traffic", element: _jsx(TrafficPage, {}) }), _jsx(Route, { path: "/settings", element: _jsx(SettingsPage, {}) }), _jsx(Route, { path: "*", element: _jsx(Navigate, { to: "/traffic", replace: true }) })] }) }));
}
