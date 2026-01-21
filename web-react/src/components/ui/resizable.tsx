import { Group, Panel, Separator, type GroupProps, type SeparatorProps } from "react-resizable-panels"

import { cn } from "@/lib/utils"

const ResizablePanelGroup = ({
  className,
  ...props
}: GroupProps) => (
  <Group
    className={cn(
      "flex h-full w-full aria-[orientation=vertical]:flex-col",
      className
    )}
    {...props}
  />
)

const ResizablePanel = Panel

const ResizableHandle = ({
  withHandle,
  className,
  ...props
}: SeparatorProps & {
  withHandle?: boolean
}) => (
  <Separator
    className={cn(
      "relative flex items-center justify-center focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring focus-visible:ring-offset-1",
      "aria-[orientation=horizontal]:h-3 aria-[orientation=horizontal]:w-full aria-[orientation=horizontal]:after:absolute aria-[orientation=horizontal]:after:inset-x-0 aria-[orientation=horizontal]:after:top-1/2 aria-[orientation=horizontal]:after:h-px aria-[orientation=horizontal]:after:-translate-y-1/2 aria-[orientation=horizontal]:after:bg-border/60",
      "aria-[orientation=vertical]:h-full aria-[orientation=vertical]:w-3 aria-[orientation=vertical]:after:absolute aria-[orientation=vertical]:after:inset-y-0 aria-[orientation=vertical]:after:left-1/2 aria-[orientation=vertical]:after:w-px aria-[orientation=vertical]:after:-translate-x-1/2 aria-[orientation=vertical]:after:bg-border/60",
      "after:transition-colors after:duration-150 hover:after:bg-accent focus-visible:after:bg-accent",
      className
    )}
    {...props}
  >
  </Separator>
)

export { ResizablePanelGroup, ResizablePanel, ResizableHandle }
