import type { ReactElement } from "react";

import {
  AppShell,
  AppShellBrand,
  AppShellContent,
  AppShellHeader,
  AppShellMain,
  AppShellNavItem,
  AppShellNavSection,
  AppShellSidebar,
  Button,
  SectionHeading,
  StatusBadge,
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardFooter,
  SurfaceCardHeader,
  SurfaceCardTitle,
} from "@compozy/ui";

function NavMark(): ReactElement {
  return <span className="size-2 rounded-full bg-current" />;
}

function FoundationStat({ label, value }: { label: string; value: string }): ReactElement {
  return (
    <div className="space-y-1 rounded-[var(--radius-md)] border border-border bg-black/10 px-4 py-3">
      <p className="font-disket text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
        {label}
      </p>
      <p className="font-display text-2xl leading-none tracking-[-0.02em] text-foreground">
        {value}
      </p>
    </div>
  );
}

export function App(): ReactElement {
  return (
    <AppShell>
      <AppShellSidebar>
        <AppShellBrand
          badge={<StatusBadge tone="accent">daemon</StatusBadge>}
          detail="localhost · operator runtime"
          title="Compozy"
        />

        <div className="space-y-5">
          <AppShellNavSection title="Workspace">
            <AppShellNavItem active badge="01" icon={<NavMark />} label="Dashboard" />
          </AppShellNavSection>

          <AppShellNavSection title="Across workflows">
            <AppShellNavItem badge="06" icon={<NavMark />} label="Workflows" />
            <AppShellNavItem badge="01" icon={<NavMark />} label="Runs" />
            <AppShellNavItem badge="09" icon={<NavMark />} label="Reviews" />
            <AppShellNavItem icon={<NavMark />} label="Memory" />
          </AppShellNavSection>
        </div>

        <SurfaceCard className="mt-auto bg-black/10">
          <SurfaceCardBody className="space-y-3">
            <p className="font-disket text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
              Foundation
            </p>
            <p className="text-sm leading-6 text-muted-foreground">
              task_03 keeps the package boundary reusable so later routes don&apos;t invent local
              tokens, layout, or badges.
            </p>
            <div className="flex flex-wrap gap-2">
              <StatusBadge tone="success">tokens</StatusBadge>
              <StatusBadge tone="info">shell</StatusBadge>
              <StatusBadge tone="warning">tests</StatusBadge>
            </div>
          </SurfaceCardBody>
        </SurfaceCard>
      </AppShellSidebar>

      <AppShellMain>
        <AppShellHeader>
          <SectionHeading
            actions={
              <>
                <Button size="sm" variant="secondary">
                  Search commands
                </Button>
                <Button size="sm">Sync all</Button>
              </>
            }
            description="The first shared UI slice is dark-first, mockup-derived, and deliberately route-agnostic."
            eyebrow="Overview"
            title="Workflow operator console"
          />
        </AppShellHeader>

        <AppShellContent className="space-y-5">
          <div className="grid gap-4 xl:grid-cols-[minmax(0,1.3fr)_minmax(0,0.9fr)]">
            <SurfaceCard>
              <SurfaceCardHeader>
                <div>
                  <SurfaceCardEyebrow>shared package</SurfaceCardEyebrow>
                  <SurfaceCardTitle>@compozy/ui</SurfaceCardTitle>
                  <SurfaceCardDescription>
                    Stable exports for tokens, typography, shell helpers, buttons, badges, and
                    content cards.
                  </SurfaceCardDescription>
                </div>
                <StatusBadge tone="info">task_03</StatusBadge>
              </SurfaceCardHeader>
              <SurfaceCardBody className="grid gap-3 sm:grid-cols-3">
                <FoundationStat label="Theme" value="dark-first" />
                <FoundationStat label="Display" value="Nippo" />
                <FoundationStat label="Mono" value="Disket" />
              </SurfaceCardBody>
              <SurfaceCardFooter>
                <p className="text-sm text-muted-foreground">
                  Token names stay semantic so later domain routes can compose UI without local hex
                  drift.
                </p>
                <Button size="sm" variant="ghost">
                  Export surface stable
                </Button>
              </SurfaceCardFooter>
            </SurfaceCard>

            <SurfaceCard>
              <SurfaceCardHeader>
                <div>
                  <SurfaceCardEyebrow>consumption path</SurfaceCardEyebrow>
                  <SurfaceCardTitle>Web imports the package directly</SurfaceCardTitle>
                  <SurfaceCardDescription>
                    The app shell preview is rendered from shared components and the package token
                    stylesheet.
                  </SurfaceCardDescription>
                </div>
                <StatusBadge tone="accent">registry</StatusBadge>
              </SurfaceCardHeader>
              <SurfaceCardBody className="space-y-3">
                <div className="rounded-[var(--radius-md)] border border-border bg-black/10 px-4 py-3">
                  <p className="font-disket text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
                    imports
                  </p>
                  <p className="mt-2 text-sm leading-6 text-foreground">
                    @compozy/ui, @compozy/ui/tokens.css, and @compozy/ui/utils
                  </p>
                </div>
                <div className="rounded-[var(--radius-md)] border border-border bg-black/10 px-4 py-3">
                  <p className="font-disket text-[10px] uppercase tracking-[0.14em] text-muted-foreground">
                    next tasks
                  </p>
                  <p className="mt-2 text-sm leading-6 text-muted-foreground">
                    Route slices can focus on data and state because the shell vocabulary now lives
                    in the shared package.
                  </p>
                </div>
              </SurfaceCardBody>
            </SurfaceCard>
          </div>
        </AppShellContent>
      </AppShellMain>
    </AppShell>
  );
}
