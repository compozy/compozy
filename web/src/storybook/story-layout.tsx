import type { ReactNode } from "react";

import { Topbar, TopbarSlotProvider } from "@agh/ui";

import { cn } from "@/lib/utils";

interface StoryFrameProps {
  children: ReactNode;
  className?: string;
}

interface StoryTopbarHostProps extends StoryFrameProps {
  title: ReactNode;
}

/** Story-only shell for components that publish route actions through useTopbarSlot. */
export function StoryTopbarHost({ children, title }: StoryTopbarHostProps) {
  return (
    <TopbarSlotProvider>
      <Topbar title={title} />
      {children}
    </TopbarSlotProvider>
  );
}

export function StorySurface({ children, className }: StoryFrameProps) {
  return <div className={cn("min-h-160 bg-background p-6 text-fg", className)}>{children}</div>;
}

export function CenteredSurface({ children, className }: StoryFrameProps) {
  return (
    <StorySurface className={cn("flex items-center justify-center", className)}>
      {children}
    </StorySurface>
  );
}

export function PanelSurface({ children, className }: StoryFrameProps) {
  return (
    <StorySurface
      className={cn(
        "flex min-h-160 overflow-hidden rounded-2xl border border-line bg-canvas p-0",
        className
      )}
    >
      {children}
    </StorySurface>
  );
}
