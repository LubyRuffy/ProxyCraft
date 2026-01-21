import { Button } from '@/components/ui/button';
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { useSettingsStore } from '@/stores/use-traffic-store';
import { Settings, RefreshCw, Save, RotateCcw } from 'lucide-react';
import { useState } from 'react';
import { Link } from 'react-router-dom';
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

  const updateTempSetting = (key: string, value: any) => {
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
    } catch (error) {
      toast.error('保存设置失败');
    } finally {
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

  return (
    <div className="flex h-full flex-col gap-4 p-4">
      {/* 头部区域 */}
      <div className="flex flex-col gap-3">
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbLink asChild>
                <Link to="/traffic">流量</Link>
              </BreadcrumbLink>
            </BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              <BreadcrumbPage>设置</BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
        <header className="flex items-center justify-between border-b pb-3">
          <div className="flex items-center gap-3">
            <Settings className="h-6 w-6" />
            <h1 className="text-2xl font-bold">设置</h1>
          </div>
          <div className="flex gap-2">
            <Button variant="outline" onClick={handleReset} disabled={!hasChanges}>
              <RotateCcw className="h-4 w-4 mr-2" />
              重置
            </Button>
            <Button onClick={handleSave} disabled={!hasChanges || isSaving}>
              <Save className="h-4 w-4 mr-2" />
              {isSaving ? '保存中...' : '保存设置'}
            </Button>
          </div>
        </header>
      </div>

      {/* 主内容区域 */}
      <div className="flex-1 overflow-auto">
        <div className="space-y-4">
          {/* 连接设置 */}
          <div className="rounded-lg border bg-card p-4 space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold">连接设置</h3>
              <p className="text-sm text-muted-foreground">配置WebSocket连接和自动重连参数</p>
            </div>

            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <Label htmlFor="auto-reconnect">自动重连</Label>
                <Switch
                  id="auto-reconnect"
                  checked={tempSettings.autoReconnect}
                  onCheckedChange={(checked) => updateTempSetting('autoReconnect', checked)}
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <Label htmlFor="reconnect-interval">重连间隔（秒）</Label>
                  <Input
                    id="reconnect-interval"
                    type="number"
                    min="1"
                    max="60"
                    value={tempSettings.reconnectInterval}
                    onChange={(e) => updateTempSetting('reconnectInterval', parseInt(e.target.value))}
                    disabled={!tempSettings.autoReconnect}
                  />
                </div>

                <div>
                  <Label htmlFor="max-attempts">最大重连次数</Label>
                  <Input
                    id="max-attempts"
                    type="number"
                    min="1"
                    max="100"
                    value={tempSettings.maxReconnectAttempts}
                    onChange={(e) => updateTempSetting('maxReconnectAttempts', parseInt(e.target.value))}
                    disabled={!tempSettings.autoReconnect}
                  />
                </div>
              </div>

              <Button variant="outline" size="sm" onClick={handleTestConnection}>
                <RefreshCw className="h-4 w-4 mr-2" />
                测试连接
              </Button>
            </div>
          </div>

          {/* 显示设置 */}
          <div className="rounded-lg border bg-card p-4 space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold">显示设置</h3>
              <p className="text-sm text-muted-foreground">自定义界面显示选项</p>
            </div>

            <div className="space-y-4">
              <div>
                <Label htmlFor="entries-per-page">每页显示条目数</Label>
                <Input
                  id="entries-per-page"
                  type="number"
                  min="10"
                  max="200"
                  value={tempSettings.entriesPerPage}
                  onChange={(e) => updateTempSetting('entriesPerPage', parseInt(e.target.value))}
                />
              </div>

              <div className="flex items-center justify-between">
                <Label htmlFor="show-only-https">仅显示HTTPS请求</Label>
                <Switch
                  id="show-only-https"
                  checked={tempSettings.showOnlyHttps}
                  onCheckedChange={(checked) => updateTempSetting('showOnlyHttps', checked)}
                />
              </div>

              <div className="flex items-center justify-between">
                <Label htmlFor="show-only-sse">仅显示SSE请求</Label>
                <Switch
                  id="show-only-sse"
                  checked={tempSettings.showOnlySse}
                  onCheckedChange={(checked) => updateTempSetting('showOnlySse', checked)}
                />
              </div>

              <div>
                <Label htmlFor="theme">主题</Label>
                <Select value={tempSettings.theme} onValueChange={(value) => updateTempSetting('theme', value as 'light' | 'dark' | 'auto')}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="light">浅色</SelectItem>
                    <SelectItem value="dark">深色</SelectItem>
                    <SelectItem value="auto">自动</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          </div>

          {/* 数据保存设置 */}
          <div className="rounded-lg border bg-card p-4 space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold">数据保存设置</h3>
              <p className="text-sm text-muted-foreground">配置HAR文件自动保存选项</p>
            </div>

            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <Label htmlFor="auto-save-har">自动保存HAR文件</Label>
                <Switch
                  id="auto-save-har"
                  checked={tempSettings.autoSaveHar}
                  onCheckedChange={(checked) => updateTempSetting('autoSaveHar', checked)}
                />
              </div>

              <div>
                <Label htmlFor="har-save-interval">保存间隔（秒）</Label>
                <Input
                  id="har-save-interval"
                  type="number"
                  min="5"
                  max="3600"
                  value={tempSettings.harSaveInterval}
                  onChange={(e) => updateTempSetting('harSaveInterval', parseInt(e.target.value))}
                  disabled={!tempSettings.autoSaveHar}
                />
              </div>
            </div>
          </div>

          {/* 过滤设置 */}
          <div className="rounded-lg border bg-card p-4 space-y-4">
            <div className="flex items-center justify-between">
              <h3 className="text-lg font-semibold">过滤设置</h3>
              <p className="text-sm text-muted-foreground">设置流量显示过滤条件</p>
            </div>

            <div className="space-y-4">
              <div>
                <Label htmlFor="filter-host">主机过滤</Label>
                <Input
                  id="filter-host"
                  placeholder="例如: example.com"
                  value={tempSettings.filterHost}
                  onChange={(e) => updateTempSetting('filterHost', e.target.value)}
                />
              </div>

              <div>
                <Label htmlFor="filter-method">HTTP方法过滤</Label>
                <Select value={tempSettings.filterMethod} onValueChange={(value) => updateTempSetting('filterMethod', value as 'all' | 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH')}>
                  <SelectTrigger>
                    <SelectValue placeholder="全部" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="all">全部</SelectItem>
                    <SelectItem value="GET">GET</SelectItem>
                    <SelectItem value="POST">POST</SelectItem>
                    <SelectItem value="PUT">PUT</SelectItem>
                    <SelectItem value="DELETE">DELETE</SelectItem>
                    <SelectItem value="PATCH">PATCH</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
