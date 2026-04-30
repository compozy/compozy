import { renderToStaticMarkup } from "react-dom/server";

import { describe, expect, expectTypeOf, it } from "vitest";

import {
  Alert,
  AppShell,
  AppShellBrand,
  AppShellContent,
  AppShellHeader,
  AppShellMain,
  AppShellNavItem,
  AppShellNavSection,
  AppShellSidebar,
  Markdown,
  Metric,
  Button,
  type ButtonProps,
  EmptyState,
  SectionHeading,
  SkeletonText,
  StatusBadge,
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardFooter,
  SurfaceCardHeader,
  SurfaceCardTitle,
  UIProvider,
  cn,
} from "../src";

function DotMark() {
  return <span className="size-2 rounded-full bg-current" />;
}

describe("shared primitives", () => {
  it("merges class names with tailwind-aware precedence", () => {
    expect(cn("px-2 py-3", undefined, "px-4")).toBe("py-3 px-4");
  });

  it("renders the initial shell and primitive foundation", () => {
    const html = renderToStaticMarkup(
      <UIProvider>
        <AppShell>
          <AppShellSidebar>
            <AppShellBrand
              badge={<StatusBadge tone="accent">daemon</StatusBadge>}
              detail="operator runtime"
              title="Compozy"
            />
            <AppShellNavSection title="Workspace">
              <AppShellNavItem active badge="01" icon={<DotMark />} label="Dashboard" />
              <AppShellNavItem badge="06" icon={<DotMark />} label="Workflows" />
            </AppShellNavSection>
          </AppShellSidebar>
          <AppShellMain>
            <AppShellHeader>
              <SectionHeading
                actions={<Button size="sm">Sync all</Button>}
                description="Shared shell primitives stay route-agnostic while matching the daemon mockup theme."
                eyebrow="Overview"
                title="Shared foundation ready"
              />
            </AppShellHeader>
            <AppShellContent>
              <EmptyState
                description="Run sync to populate this workspace."
                title="Nothing to show yet"
              />
              <SurfaceCard>
                <SurfaceCardHeader>
                  <div>
                    <SurfaceCardEyebrow>tokens</SurfaceCardEyebrow>
                    <SurfaceCardTitle>Mockup-derived theme</SurfaceCardTitle>
                    <SurfaceCardDescription>
                      Self-hosted display and mono fonts plus dark-first semantic tokens.
                    </SurfaceCardDescription>
                  </div>
                  <StatusBadge tone="info">task_03</StatusBadge>
                </SurfaceCardHeader>
                <SurfaceCardBody>
                  <Button variant="secondary">Search commands</Button>
                </SurfaceCardBody>
                <SurfaceCardFooter>
                  <StatusBadge tone="success">stable</StatusBadge>
                </SurfaceCardFooter>
              </SurfaceCard>
            </AppShellContent>
          </AppShellMain>
        </AppShell>
      </UIProvider>
    );

    expect(html).toContain("Compozy");
    expect(html).toContain("operator runtime");
    expect(html).toContain("Shared foundation ready");
    expect(html).toContain("Mockup-derived theme");
    expect(html).toContain("Nothing to show yet");
    expect(html).toContain("Search commands");
    expect(html).toContain("stable");
    expect(html).toContain('aria-current="page"');
  });

  it("renders optional branches for minimal shell usage", () => {
    const html = renderToStaticMarkup(
      <AppShell>
        <AppShellSidebar>
          <AppShellBrand title="Compozy" />
          <AppShellNavSection>
            <AppShellNavItem icon={<DotMark />} label="Runs" />
          </AppShellNavSection>
        </AppShellSidebar>
        <AppShellMain>
          <AppShellHeader>
            <SectionHeading title="Runs console" />
          </AppShellHeader>
          <AppShellContent>
            <SurfaceCard>
              <SurfaceCardBody>
                <Button icon={<DotMark />}>Run now</Button>
              </SurfaceCardBody>
            </SurfaceCard>
          </AppShellContent>
        </AppShellMain>
      </AppShell>
    );

    expect(html).toContain("Runs console");
    expect(html).toContain("Run now");
    expect(html).not.toContain('aria-current="page"');
  });

  it("keeps one-child surface card slots shrinkable", () => {
    const html = renderToStaticMarkup(
      <SurfaceCard>
        <SurfaceCardHeader>
          <div>Header content</div>
        </SurfaceCardHeader>
        <SurfaceCardFooter>
          <div>Footer content</div>
        </SurfaceCardFooter>
      </SurfaceCard>
    );

    expect(html).toContain("[&amp;&gt;*:last-child:not(:only-child)]:shrink-0");
    expect(html).not.toContain("[&amp;&gt;*:last-child]:shrink-0");
  });

  it("requires an accessible name for icon-only buttons", () => {
    const labeledIconOnly = {
      icon: <DotMark />,
      "aria-label": "Refresh runs",
    } satisfies ButtonProps;
    const labelledByIconOnly = {
      icon: <DotMark />,
      "aria-labelledby": "refresh-runs-label",
    } satisfies ButtonProps;

    expectTypeOf(labeledIconOnly).toMatchTypeOf<ButtonProps>();
    expectTypeOf(labelledByIconOnly).toMatchTypeOf<ButtonProps>();

    // @ts-expect-error icon-only buttons require aria-label or aria-labelledby
    const unlabeledIconOnly: ButtonProps = { icon: <DotMark /> };

    const html = renderToStaticMarkup(<Button {...labeledIconOnly} />);

    expect(html).toContain('aria-label="Refresh runs"');
    expect(unlabeledIconOnly.icon).toBeDefined();
  });

  it("renders loading buttons as busy and disabled", () => {
    const html = renderToStaticMarkup(<Button loading>Resolve workspace</Button>);

    expect(html).toContain('aria-busy="true"');
    expect(html).toMatch(/<button[^>]*disabled=""/);
    expect(html).toContain("Resolve workspace");
  });

  it("only makes error alerts live regions by default", () => {
    const infoHtml = renderToStaticMarkup(<Alert variant="info">Heads up</Alert>);
    const errorHtml = renderToStaticMarkup(<Alert variant="error">Broken</Alert>);

    expect(infoHtml).not.toContain('role="status"');
    expect(infoHtml).not.toContain('role="alert"');
    expect(errorHtml).toContain('role="alert"');
  });

  it("targets markdown link hover styles on the anchor itself", () => {
    const html = renderToStaticMarkup(<Markdown>[Docs](https://example.com)</Markdown>);

    expect(html).toContain("[&amp;_a:hover]:brightness-110");
    expect(html).not.toContain("hover:[&amp;_a]:brightness-110");
  });

  it("renders metric slots without paragraph wrappers", () => {
    const html = renderToStaticMarkup(
      <Metric hint={<div>delta</div>} label={<div>throughput</div>} value={<div>42</div>} />
    );

    expect(html).toContain("<div>throughput</div>");
    expect(html).toContain("<div>42</div>");
    expect(html).toContain("<div>delta</div>");
    expect(html).not.toContain("<p");
  });

  it("normalizes skeleton text underflow so the last row keeps its width treatment", () => {
    const html = renderToStaticMarkup(<SkeletonText lines={0} />);

    expect(html).toContain("w-2/3");
  });

  it("gates pulsing status dots behind motion-safe", () => {
    const html = renderToStaticMarkup(<StatusBadge pulse>running</StatusBadge>);

    expect(html).toContain("motion-safe:animate-pulse");
  });
});
