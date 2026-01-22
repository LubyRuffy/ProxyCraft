import { useMemo } from 'react';
import { Outlet, NavLink } from 'react-router-dom';
import { Badge } from '@/components/ui/badge';
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from '@/components/ui/resizable';
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarRail,
  useSidebar,
} from '@/components/ui/sidebar';
import { Toggle } from '@/components/ui/toggle';
import { cn } from '@/lib/utils';
import { SettingsPage } from '@/pages/settings';
import { useLayoutStore } from '@/stores/use-layout-store';
import { Compass, PanelBottom, PanelLeft, PanelRight } from 'lucide-react';

const navItems = [
  { label: '流量列表', to: '/traffic', icon: Compass },
];

export function AppLayout() {
  return (
    <SidebarProvider defaultOpen={false} className="w-full min-w-0 overflow-x-hidden">
      <AppLayoutContent />
    </SidebarProvider>
  );
}

function AppLayoutContent() {
  const { isMobile, open, openMobile, toggleSidebar } = useSidebar();
  const showDetail = useLayoutStore((state) => state.showDetail);
  const showSettings = useLayoutStore((state) => state.showSettings);
  const setShowDetail = useLayoutStore((state) => state.setShowDetail);
  const setShowSettings = useLayoutStore((state) => state.setShowSettings);

  const socketLabel = useMemo(() => {
    const envUrl = import.meta.env.VITE_PROXYCRAFT_SOCKET_URL;
    const fallback = typeof window !== 'undefined' ? window.location.origin : '';
    const base = envUrl || fallback;
    if (!base) return 'Auto';
    try {
      return new URL(base).host;
    } catch (error) {
      return base;
    }
  }, []);

  const sidebarPressed = isMobile ? openMobile : open;
  return (
    <>
      <Sidebar className="border-sidebar-border/60 bg-sidebar/95">
        <SidebarContent className="px-3 py-3 text-sm group-data-[state=collapsed]:px-2">
          <SidebarGroup>
            <SidebarGroupLabel>Apps</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu className="group-data-[state=collapsed]:items-center group-data-[state=collapsed]:gap-2">
                {navItems.map((item) => (
                  <SidebarMenuItem key={item.to}>
                    <SidebarMenuButton asChild tooltip={item.label}>
                      <NavLink
                        to={item.to}
                        className={({ isActive }) =>
                          cn(isActive && 'bg-sidebar-accent text-sidebar-accent-foreground')
                        }
                        end
                      >
                        <item.icon className="h-4 w-4" />
                        <span className="leading-tight group-data-[state=collapsed]:hidden">{item.label}</span>
                      </NavLink>
                    </SidebarMenuButton>
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>
        <SidebarFooter className="border-t border-sidebar-border/60 px-3 py-3 text-xs text-sidebar-foreground/60 group-data-[state=collapsed]:hidden">
          <div className="flex items-center justify-between">
            <span>Local Proxy</span>
            <Badge variant="outline">Active</Badge>
          </div>
        </SidebarFooter>
        <SidebarRail />
      </Sidebar>

      <SidebarInset className="w-full min-h-0 min-w-0 overflow-x-hidden">
        <header className="border-b border-border/60 bg-card/80">
          <div className="flex h-9 items-center justify-between px-3 text-xs">
            <div className="flex items-center gap-3">
              <span className="text-muted-foreground">{socketLabel}</span>
            </div>
            <div className="flex items-center gap-1.5">
              <Toggle
                variant="outline"
                size="sm"
                pressed={sidebarPressed}
                onPressedChange={() => toggleSidebar()}
                aria-label="切换侧边栏"
                title="侧边栏"
              >
                <PanelLeft className="h-4 w-4" />
              </Toggle>
              <Toggle
                variant="outline"
                size="sm"
                pressed={showDetail}
                onPressedChange={setShowDetail}
                aria-label="切换详情面板"
                title="详情面板"
              >
                <PanelBottom className="h-4 w-4" />
              </Toggle>
              <Toggle
                variant="outline"
                size="sm"
                pressed={showSettings}
                onPressedChange={setShowSettings}
                aria-label="切换设置面板"
                title="设置面板"
              >
                <PanelRight className="h-4 w-4" />
              </Toggle>
            </div>
          </div>
        </header>
        <ResizablePanelGroup
          orientation="horizontal"
          className="flex-1 min-h-0 w-full min-w-0 overflow-hidden"
        >
          <ResizablePanel
            defaultSize={showSettings ? 80 : 100}
            minSize={35}
            className="flex min-h-0 min-w-0 flex-col"
          >
            <main className="flex h-full min-h-0 w-full min-w-0 flex-1 flex-col overflow-x-hidden">
              <div className="flex min-h-0 w-full min-w-0 flex-1 flex-col">
                <Outlet />
              </div>
            </main>
          </ResizablePanel>
          {showSettings ? (
            <>
              <ResizableHandle className="bg-border/60 transition-colors hover:bg-accent/60 focus:outline-none" />
              <ResizablePanel
                defaultSize={20}
                minSize={20}
                className="flex min-h-0 min-w-0 flex-col border-l border-border/60 bg-card/70"
              >
                <div className="flex h-full min-h-0 min-w-0 flex-col overflow-hidden">
                  <SettingsPage />
                </div>
              </ResizablePanel>
            </>
          ) : null}
        </ResizablePanelGroup>
      </SidebarInset>
    </>
  );
}
