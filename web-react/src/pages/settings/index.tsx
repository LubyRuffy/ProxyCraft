import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Field, FieldContent, FieldGroup, FieldLabel } from '@/components/ui/field';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { useSettingsStore } from '@/stores/use-traffic-store';
import { Settings, RotateCcw } from 'lucide-react';
import { toast } from 'sonner';

export function SettingsPage() {
  const settings = useSettingsStore();

  const handleReset = () => {
    settings.resetSettings();
    toast.success('设置已重置');
  };

  return (
    <div className="flex h-full flex-col gap-4 p-4">
      {/* 头部区域 */}
      <div className="flex flex-col gap-3">
        <header className="flex items-center justify-between border-b pb-3">
          <div className="flex items-center gap-3">
            <Settings className="h-6 w-6" />
            <h3 className="text-2xl font-bold">设置</h3>
          </div>
          <div className="flex gap-2">
            <Button variant="outline" onClick={handleReset}>
              <RotateCcw className="h-4 w-4 mr-2" />
              重置
            </Button>
          </div>
        </header>
      </div>

      {/* 主内容区域 */}
      <div className="flex-1 overflow-auto">
        <div className="space-y-4">
          {/* 主题设置 */}
          <Card>
            <CardHeader>
              <CardTitle>主题设置</CardTitle>
              <CardDescription>选择界面主题样式</CardDescription>
            </CardHeader>
            <CardContent>
              <FieldGroup>
                <Field>
                  <FieldLabel htmlFor="theme-trigger">主题</FieldLabel>
                  <FieldContent>
                    <Select
                      value={settings.theme}
                      onValueChange={(value) => settings.setTheme(value as 'light' | 'dark' | 'auto')}
                    >
                      <SelectTrigger id="theme-trigger">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="light">浅色</SelectItem>
                        <SelectItem value="dark">深色</SelectItem>
                        <SelectItem value="auto">自动</SelectItem>
                      </SelectContent>
                    </Select>
                  </FieldContent>
                </Field>
              </FieldGroup>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
