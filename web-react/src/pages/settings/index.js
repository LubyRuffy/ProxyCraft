import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { useSettingsStore } from '@/stores/use-traffic-store';
import { Settings, RefreshCw, Save, RotateCcw } from 'lucide-react';
import { useState } from 'react';
import { toast } from 'sonner';
export function SettingsPage() {
    const settings = useSettingsStore();
    const [tempSettings, setTempSettings] = useState({
        autoReconnect: settings.autoReconnect,
        reconnectInterval: settings.reconnectInterval,
        maxReconnectAttempts: settings.maxReconnectAttempts,
        entriesPerPage: settings.entriesPerPage,
        showOnlyHttps: settings.showOnlyHttps,
        showOnlySse: settings.showOnlySse,
        autoSaveHar: settings.autoSaveHar,
        harSaveInterval: settings.harSaveInterval,
        filterHost: settings.filterHost,
        filterMethod: settings.filterMethod,
        theme: settings.theme,
    });
    const [hasChanges, setHasChanges] = useState(false);
    const [isSaving, setIsSaving] = useState(false);
    const updateTempSetting = (key, value) => {
        setTempSettings(prev => ({ ...prev, [key]: value }));
        setHasChanges(true);
    };
    const handleSave = async () => {
        setIsSaving(true);
        try {
            // 这里应该发送设置到后端API
            // 暂时先更新本地状态
            settings.setAutoReconnect(tempSettings.autoReconnect);
            settings.setReconnectInterval(tempSettings.reconnectInterval);
            settings.setMaxReconnectAttempts(tempSettings.maxReconnectAttempts);
            settings.setEntriesPerPage(tempSettings.entriesPerPage);
            settings.setShowOnlyHttps(tempSettings.showOnlyHttps);
            settings.setShowOnlySse(tempSettings.showOnlySse);
            settings.setAutoSaveHar(tempSettings.autoSaveHar);
            settings.setHarSaveInterval(tempSettings.harSaveInterval);
            settings.setFilterHost(tempSettings.filterHost);
            settings.setFilterMethod(tempSettings.filterMethod);
            settings.setTheme(tempSettings.theme);
            setHasChanges(false);
            toast.success('设置已保存');
        }
        catch (error) {
            toast.error('保存设置失败');
        }
        finally {
            setIsSaving(false);
        }
    };
    const handleReset = () => {
        settings.resetSettings();
        setTempSettings({
            autoReconnect: settings.autoReconnect,
            reconnectInterval: settings.reconnectInterval,
            maxReconnectAttempts: settings.maxReconnectAttempts,
            entriesPerPage: settings.entriesPerPage,
            showOnlyHttps: settings.showOnlyHttps,
            showOnlySse: settings.showOnlySse,
            autoSaveHar: settings.autoSaveHar,
            harSaveInterval: settings.harSaveInterval,
            filterHost: settings.filterHost,
            filterMethod: settings.filterMethod,
            theme: settings.theme,
        });
        setHasChanges(false);
        toast.success('设置已重置');
    };
    const handleTestConnection = () => {
        toast.info('测试连接功能待实现');
    };
    return (_jsxs("div", { className: "container mx-auto p-6", children: [_jsxs("div", { className: "flex items-center gap-2 mb-6", children: [_jsx(Settings, { className: "h-6 w-6" }), _jsx("h1", { className: "text-2xl font-bold", children: "\u8BBE\u7F6E" })] }), _jsxs("div", { className: "space-y-6", children: [_jsxs(Card, { children: [_jsxs(CardHeader, { children: [_jsx(CardTitle, { children: "\u8FDE\u63A5\u8BBE\u7F6E" }), _jsx(CardDescription, { children: "\u914D\u7F6EWebSocket\u8FDE\u63A5\u548C\u81EA\u52A8\u91CD\u8FDE\u53C2\u6570" })] }), _jsxs(CardContent, { className: "space-y-4", children: [_jsxs("div", { className: "flex items-center justify-between", children: [_jsx(Label, { htmlFor: "auto-reconnect", children: "\u81EA\u52A8\u91CD\u8FDE" }), _jsx(Switch, { id: "auto-reconnect", checked: tempSettings.autoReconnect, onCheckedChange: (checked) => updateTempSetting('autoReconnect', checked) })] }), _jsxs("div", { className: "grid grid-cols-2 gap-4", children: [_jsxs("div", { children: [_jsx(Label, { htmlFor: "reconnect-interval", children: "\u91CD\u8FDE\u95F4\u9694\uFF08\u79D2\uFF09" }), _jsx(Input, { id: "reconnect-interval", type: "number", min: "1", max: "60", value: tempSettings.reconnectInterval, onChange: (e) => updateTempSetting('reconnectInterval', parseInt(e.target.value)), disabled: !tempSettings.autoReconnect })] }), _jsxs("div", { children: [_jsx(Label, { htmlFor: "max-attempts", children: "\u6700\u5927\u91CD\u8FDE\u6B21\u6570" }), _jsx(Input, { id: "max-attempts", type: "number", min: "1", max: "100", value: tempSettings.maxReconnectAttempts, onChange: (e) => updateTempSetting('maxReconnectAttempts', parseInt(e.target.value)), disabled: !tempSettings.autoReconnect })] })] }), _jsxs(Button, { variant: "outline", size: "sm", onClick: handleTestConnection, children: [_jsx(RefreshCw, { className: "h-4 w-4 mr-2" }), "\u6D4B\u8BD5\u8FDE\u63A5"] })] })] }), _jsxs(Card, { children: [_jsxs(CardHeader, { children: [_jsx(CardTitle, { children: "\u663E\u793A\u8BBE\u7F6E" }), _jsx(CardDescription, { children: "\u81EA\u5B9A\u4E49\u754C\u9762\u663E\u793A\u9009\u9879" })] }), _jsxs(CardContent, { className: "space-y-4", children: [_jsxs("div", { children: [_jsx(Label, { htmlFor: "entries-per-page", children: "\u6BCF\u9875\u663E\u793A\u6761\u76EE\u6570" }), _jsx(Input, { id: "entries-per-page", type: "number", min: "10", max: "200", value: tempSettings.entriesPerPage, onChange: (e) => updateTempSetting('entriesPerPage', parseInt(e.target.value)) })] }), _jsxs("div", { className: "flex items-center justify-between", children: [_jsx(Label, { htmlFor: "show-only-https", children: "\u4EC5\u663E\u793AHTTPS\u8BF7\u6C42" }), _jsx(Switch, { id: "show-only-https", checked: tempSettings.showOnlyHttps, onCheckedChange: (checked) => updateTempSetting('showOnlyHttps', checked) })] }), _jsxs("div", { className: "flex items-center justify-between", children: [_jsx(Label, { htmlFor: "show-only-sse", children: "\u4EC5\u663E\u793ASSE\u8BF7\u6C42" }), _jsx(Switch, { id: "show-only-sse", checked: tempSettings.showOnlySse, onCheckedChange: (checked) => updateTempSetting('showOnlySse', checked) })] }), _jsxs("div", { children: [_jsx(Label, { htmlFor: "theme", children: "\u4E3B\u9898" }), _jsxs(Select, { value: tempSettings.theme, onValueChange: (value) => updateTempSetting('theme', value), children: [_jsx(SelectTrigger, { children: _jsx(SelectValue, {}) }), _jsxs(SelectContent, { children: [_jsx(SelectItem, { value: "light", children: "\u6D45\u8272" }), _jsx(SelectItem, { value: "dark", children: "\u6DF1\u8272" }), _jsx(SelectItem, { value: "auto", children: "\u81EA\u52A8" })] })] })] })] })] }), _jsxs(Card, { children: [_jsxs(CardHeader, { children: [_jsx(CardTitle, { children: "\u6570\u636E\u4FDD\u5B58\u8BBE\u7F6E" }), _jsx(CardDescription, { children: "\u914D\u7F6EHAR\u6587\u4EF6\u81EA\u52A8\u4FDD\u5B58\u9009\u9879" })] }), _jsxs(CardContent, { className: "space-y-4", children: [_jsxs("div", { className: "flex items-center justify-between", children: [_jsx(Label, { htmlFor: "auto-save-har", children: "\u81EA\u52A8\u4FDD\u5B58HAR\u6587\u4EF6" }), _jsx(Switch, { id: "auto-save-har", checked: tempSettings.autoSaveHar, onCheckedChange: (checked) => updateTempSetting('autoSaveHar', checked) })] }), _jsxs("div", { children: [_jsx(Label, { htmlFor: "har-save-interval", children: "\u4FDD\u5B58\u95F4\u9694\uFF08\u79D2\uFF09" }), _jsx(Input, { id: "har-save-interval", type: "number", min: "5", max: "3600", value: tempSettings.harSaveInterval, onChange: (e) => updateTempSetting('harSaveInterval', parseInt(e.target.value)), disabled: !tempSettings.autoSaveHar })] })] })] }), _jsxs(Card, { children: [_jsxs(CardHeader, { children: [_jsx(CardTitle, { children: "\u8FC7\u6EE4\u8BBE\u7F6E" }), _jsx(CardDescription, { children: "\u8BBE\u7F6E\u6D41\u91CF\u663E\u793A\u8FC7\u6EE4\u6761\u4EF6" })] }), _jsxs(CardContent, { className: "space-y-4", children: [_jsxs("div", { children: [_jsx(Label, { htmlFor: "filter-host", children: "\u4E3B\u673A\u8FC7\u6EE4" }), _jsx(Input, { id: "filter-host", placeholder: "\u4F8B\u5982: example.com", value: tempSettings.filterHost, onChange: (e) => updateTempSetting('filterHost', e.target.value) })] }), _jsxs("div", { children: [_jsx(Label, { htmlFor: "filter-method", children: "HTTP\u65B9\u6CD5\u8FC7\u6EE4" }), _jsxs(Select, { value: tempSettings.filterMethod, onValueChange: (value) => updateTempSetting('filterMethod', value), children: [_jsx(SelectTrigger, { children: _jsx(SelectValue, { placeholder: "\u5168\u90E8" }) }), _jsxs(SelectContent, { children: [_jsx(SelectItem, { value: "all", children: "\u5168\u90E8" }), _jsx(SelectItem, { value: "GET", children: "GET" }), _jsx(SelectItem, { value: "POST", children: "POST" }), _jsx(SelectItem, { value: "PUT", children: "PUT" }), _jsx(SelectItem, { value: "DELETE", children: "DELETE" }), _jsx(SelectItem, { value: "PATCH", children: "PATCH" })] })] })] })] })] }), _jsx(Separator, {}), _jsxs("div", { className: "flex justify-end gap-2", children: [_jsxs(Button, { variant: "outline", onClick: handleReset, disabled: !hasChanges, children: [_jsx(RotateCcw, { className: "h-4 w-4 mr-2" }), "\u91CD\u7F6E"] }), _jsxs(Button, { onClick: handleSave, disabled: !hasChanges || isSaving, children: [_jsx(Save, { className: "h-4 w-4 mr-2" }), isSaving ? '保存中...' : '保存设置'] })] })] })] }));
}
