import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';
import { cva, type VariantProps } from 'class-variance-authority';

import { cn } from '@/lib/utils';

type SidebarState = 'expanded' | 'collapsed';

interface SidebarContextValue {
  state: SidebarState;
  open: boolean;
  setOpen: React.Dispatch<React.SetStateAction<boolean>>;
  openMobile: boolean;
  setOpenMobile: React.Dispatch<React.SetStateAction<boolean>>;
  isDesktop: boolean;
  toggleSidebar: () => void;
}

const SidebarContext = React.createContext<SidebarContextValue | null>(null);

function useSidebar() {
  const context = React.useContext(SidebarContext);
  if (!context) {
    throw new Error('useSidebar must be used within a SidebarProvider.');
  }
  return context;
}

function useMediaQuery(query: string) {
  const [matches, setMatches] = React.useState(false);

  React.useEffect(() => {
    if (typeof window === 'undefined') return;
    const media = window.matchMedia(query);
    const handler = () => setMatches(media.matches);
    handler();
    if (media.addEventListener) {
      media.addEventListener('change', handler);
    } else {
      media.addListener(handler);
    }
    return () => {
      if (media.removeEventListener) {
        media.removeEventListener('change', handler);
      } else {
        media.removeListener(handler);
      }
    };
  }, [query]);

  return matches;
}

interface SidebarProviderProps extends React.HTMLAttributes<HTMLDivElement> {
  defaultOpen?: boolean;
}

const SidebarProvider = React.forwardRef<HTMLDivElement, SidebarProviderProps>(
  ({ className, defaultOpen = true, children, ...props }, ref) => {
    const isDesktop = useMediaQuery('(min-width: 768px)');
    const [open, setOpen] = React.useState(defaultOpen);
    const [openMobile, setOpenMobile] = React.useState(false);

    const state: SidebarState = open ? 'expanded' : 'collapsed';
    const toggleSidebar = React.useCallback(() => {
      if (isDesktop) {
        setOpen((prev) => !prev);
      } else {
        setOpenMobile((prev) => !prev);
      }
    }, [isDesktop]);

    const value = React.useMemo(
      () => ({ state, open, setOpen, openMobile, setOpenMobile, isDesktop, toggleSidebar }),
      [state, open, openMobile, isDesktop, toggleSidebar]
    );

    return (
      <SidebarContext.Provider value={value}>
        <div
          ref={ref}
          className={cn('flex min-h-screen w-full bg-background text-foreground', className)}
          data-sidebar="wrapper"
          data-state={state}
          {...props}
        >
          {children}
        </div>
      </SidebarContext.Provider>
    );
  }
);
SidebarProvider.displayName = 'SidebarProvider';

interface SidebarProps extends React.HTMLAttributes<HTMLDivElement> {
  collapsible?: 'icon' | 'offcanvas' | 'none';
}

const Sidebar = React.forwardRef<HTMLDivElement, SidebarProps>(
  ({ className, collapsible = 'icon', ...props }, ref) => {
    const { open, openMobile, isDesktop } = useSidebar();
    const active = isDesktop ? open : openMobile;

    return (
      <div
        ref={ref}
        className={cn(
          'group relative flex h-screen w-64 min-w-64 shrink-0 flex-col overflow-hidden border-r border-sidebar-border bg-sidebar text-sidebar-foreground transition-[width] duration-200 ease-linear data-[state=collapsed]:w-14 data-[state=collapsed]:min-w-14 data-[state=collapsed]:max-w-14',
          className
        )}
        data-sidebar="sidebar"
        data-collapsible={collapsible}
        data-state={active ? 'expanded' : 'collapsed'}
        {...props}
      />
    );
  }
);
Sidebar.displayName = 'Sidebar';

const SidebarHeader = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn('flex flex-col gap-2', className)} data-sidebar="header" {...props} />
  )
);
SidebarHeader.displayName = 'SidebarHeader';

const SidebarContent = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      className={cn('flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto', className)}
      data-sidebar="content"
      {...props}
    />
  )
);
SidebarContent.displayName = 'SidebarContent';

const SidebarFooter = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn('flex flex-col gap-2', className)} data-sidebar="footer" {...props} />
  )
);
SidebarFooter.displayName = 'SidebarFooter';

const SidebarInset = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      className={cn('flex min-h-screen flex-1 flex-col bg-background text-foreground', className)}
      data-sidebar="inset"
      {...props}
    />
  )
);
SidebarInset.displayName = 'SidebarInset';

interface SidebarTriggerProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  asChild?: boolean;
}

const SidebarTrigger = React.forwardRef<HTMLButtonElement, SidebarTriggerProps>(
  ({ className, asChild = false, onClick, ...props }, ref) => {
    const { toggleSidebar } = useSidebar();
    const Comp = asChild ? Slot : 'button';

    return (
      <Comp
        ref={ref}
        type="button"
        onClick={(event: React.MouseEvent<HTMLButtonElement>) => {
          onClick?.(event);
          toggleSidebar();
        }}
        className={cn(
          'inline-flex h-8 w-8 items-center justify-center rounded-md border border-sidebar-border bg-sidebar text-sidebar-foreground transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background',
          className
        )}
        data-sidebar="trigger"
        {...props}
      />
    );
  }
);
SidebarTrigger.displayName = 'SidebarTrigger';

const SidebarRail = React.forwardRef<HTMLButtonElement, React.ButtonHTMLAttributes<HTMLButtonElement>>(
  ({ className, ...props }, ref) => {
    const { toggleSidebar } = useSidebar();
    return (
      <button
        ref={ref}
        type="button"
        aria-label="Toggle sidebar"
        onClick={toggleSidebar}
        className={cn(
          'absolute right-0 top-0 flex h-full w-4 translate-x-1/2 items-center justify-center opacity-0 transition-opacity duration-200 ease-linear group-data-[state=expanded]:opacity-100 group-data-[state=collapsed]:pointer-events-none',
          className
        )}
        data-sidebar="rail"
        {...props}
      >
        <span className="h-10 w-0.5 rounded-full bg-sidebar-border" />
      </button>
    );
  }
);
SidebarRail.displayName = 'SidebarRail';

const SidebarGroup = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn('flex flex-col gap-3', className)} data-sidebar="group" {...props} />
  )
);
SidebarGroup.displayName = 'SidebarGroup';

const SidebarGroupLabel = React.forwardRef<HTMLParagraphElement, React.HTMLAttributes<HTMLParagraphElement>>(
  ({ className, ...props }, ref) => (
    <p
      ref={ref}
      className={cn(
        'text-[11px] uppercase tracking-[0.25em] text-sidebar-foreground/60 group-data-[state=collapsed]:sr-only',
        className
      )}
      data-sidebar="group-label"
      {...props}
    />
  )
);
SidebarGroupLabel.displayName = 'SidebarGroupLabel';

const SidebarGroupContent = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn('flex flex-col gap-2', className)} data-sidebar="group-content" {...props} />
  )
);
SidebarGroupContent.displayName = 'SidebarGroupContent';

const SidebarMenu = React.forwardRef<HTMLUListElement, React.HTMLAttributes<HTMLUListElement>>(
  ({ className, ...props }, ref) => (
    <ul ref={ref} className={cn('flex flex-col gap-1', className)} data-sidebar="menu" {...props} />
  )
);
SidebarMenu.displayName = 'SidebarMenu';

const SidebarMenuItem = React.forwardRef<HTMLLIElement, React.HTMLAttributes<HTMLLIElement>>(
  ({ className, ...props }, ref) => (
    <li ref={ref} className={cn('flex', className)} data-sidebar="menu-item" {...props} />
  )
);
SidebarMenuItem.displayName = 'SidebarMenuItem';

const sidebarMenuButtonVariants = cva(
  'group/menu-button flex w-full items-center gap-2 rounded-lg px-3 py-2 text-[13px] text-sidebar-foreground/70 outline-none ring-sidebar-ring transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 data-[active=true]:bg-sidebar-accent data-[active=true]:text-sidebar-accent-foreground disabled:pointer-events-none disabled:opacity-50 group-data-[state=collapsed]:h-10 group-data-[state=collapsed]:w-10 group-data-[state=collapsed]:justify-center group-data-[state=collapsed]:px-0',
  {
    variants: {
      size: {
        default: 'h-9',
        sm: 'h-8 text-xs',
        lg: 'h-10 text-sm',
      },
    },
    defaultVariants: {
      size: 'default',
    },
  }
);

interface SidebarMenuButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof sidebarMenuButtonVariants> {
  asChild?: boolean;
  isActive?: boolean;
  tooltip?: string;
}

const SidebarMenuButton = React.forwardRef<HTMLButtonElement, SidebarMenuButtonProps>(
  ({ className, size, asChild = false, isActive, tooltip, ...props }, ref) => {
    const { state, isDesktop } = useSidebar();
    const Comp = asChild ? Slot : 'button';
    const showTooltip = Boolean(tooltip) && state === 'collapsed' && isDesktop;

    if (showTooltip) {
      return (
        <div className="group/sidebar-tooltip relative flex" data-sidebar="menu-button-tooltip">
          <Comp
            ref={ref}
            className={cn(sidebarMenuButtonVariants({ size, className }))}
            data-active={isActive}
            aria-label={tooltip}
            data-sidebar="menu-button"
            {...props}
          />
          <span className="pointer-events-none absolute left-full top-1/2 z-50 ml-3 -translate-y-1/2 whitespace-nowrap rounded-md border border-sidebar-border bg-sidebar px-2 py-1 text-xs text-sidebar-foreground opacity-0 shadow-md transition-opacity duration-200 group-hover/sidebar-tooltip:opacity-100 group-focus-within/sidebar-tooltip:opacity-100">
            {tooltip}
          </span>
        </div>
      );
    }

    return (
      <Comp
        ref={ref}
        className={cn(sidebarMenuButtonVariants({ size, className }))}
        data-active={isActive}
        data-sidebar="menu-button"
        {...props}
      />
    );
  }
);
SidebarMenuButton.displayName = 'SidebarMenuButton';

export {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarRail,
  SidebarTrigger,
  useSidebar,
};
