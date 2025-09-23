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

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground">
      <header className="border-b bg-card/70 backdrop-blur">
        <div className="mx-auto flex h-14 w-full items-center justify-between px-6">
          <div className="flex items-center gap-6">
            <span className="text-sm font-semibold tracking-tight">ProxyCraft Console</span>
            <nav className="flex items-center gap-3 text-sm text-muted-foreground">
              {navItems.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  className={({ isActive }) =>
                    cn(
                      'rounded-md px-2 py-1 transition-colors hover:text-foreground',
                      isActive && 'bg-secondary text-secondary-foreground'
                    )
                  }
                  end
                >
                  {item.label}
                </NavLink>
              ))}
            </nav>
          </div>
          <Button variant="outline" size="sm" onClick={handleSettingsClick}>
            <Settings className="h-4 w-4 mr-2" />
            设置
          </Button>
        </div>
      </header>
      <main className="flex-1">
        <Outlet />
      </main>
    </div>
  );
}
