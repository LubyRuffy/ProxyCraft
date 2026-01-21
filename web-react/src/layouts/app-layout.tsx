import { useMemo } from 'react';
import { Outlet, NavLink, useNavigate } from 'react-router-dom';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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
  SidebarTrigger,
} from '@/components/ui/sidebar';
import { cn } from '@/lib/utils';
import { useTrafficStore } from '@/stores/use-traffic-store';
import { Compass, PanelLeft, Settings } from 'lucide-react';

const navItems = [
  { label: '流量列表', to: '/traffic', icon: Compass },
];

export function AppLayout() {
  const navigate = useNavigate();
  const connected = useTrafficStore((state) => state.connected);
  const transport = useTrafficStore((state) => state.transport);

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

  const transportLabel = useMemo(() => (transport ? transport.toUpperCase() : 'UNKNOWN'), [transport]);

  const handleSettingsClick = () => {
    navigate('/settings');
  };

  return (
        <SidebarProvider defaultOpen className="w-full min-w-0 overflow-x-hidden">
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

        <SidebarInset className="w-full min-w-0 overflow-x-hidden">
          <header className="border-b border-border/60 bg-card/80">
            <div className="flex h-9 items-center justify-between px-3 text-xs">
              <div className="flex items-center gap-3">
                <SidebarTrigger className="h-7 w-7">
                  <PanelLeft className="h-4 w-4" />
                </SidebarTrigger>
                <Badge variant={connected ? 'success' : 'warning'}>{connected ? 'Connected' : 'Disconnected'}</Badge>
                <span className="text-muted-foreground">{socketLabel}</span>
              </div>
              <div className="flex items-center gap-2.5">
                <div className="flex items-center gap-1.5 text-muted-foreground">
                  <span>Transport</span>
                  <Badge variant="secondary">{transportLabel}</Badge>
                </div>
                <Button variant="outline" size="sm" onClick={handleSettingsClick} className="h-6 px-2.5 text-xs">
                  <Settings className="mr-2 h-4 w-4" />
                  设置
                </Button>
              </div>
            </div>
          </header>
        <main className="flex w-full min-w-0 flex-1 min-h-0 flex-col overflow-x-hidden">
          <div className="flex w-full min-w-0 flex-1 min-h-0 flex-col">
            <Outlet />
          </div>
        </main>
        </SidebarInset>
      </SidebarProvider>
  );
}
