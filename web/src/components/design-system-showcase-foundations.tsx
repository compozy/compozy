import { Card, CardContent, CardHeader, CardTitle, Eyebrow, Pill, Section } from "@agh/ui";

import { SectionLink } from "./design-system-showcase-section-link";
import { sectionById } from "./design-system-showcase-sections";
import { TOKEN_GROUPS } from "./design-system-showcase-tokens";
import type { TokenSwatch } from "./design-system-showcase-tokens";

export function FoundationsTokenSection() {
  return (
    <Section
      id="foundations"
      data-testid="section-foundations"
      label={<SectionLink section={sectionById("foundations")}>Foundations: Tokens</SectionLink>}
      right={<Pill mono>tokens.css</Pill>}
    >
      <div className="flex flex-col gap-6 pt-4">
        {TOKEN_GROUPS.map(group => (
          <div
            key={group.id}
            data-testid={`token-group-${group.id}`}
            data-group={group.id}
            className="flex flex-col gap-3"
          >
            <header className="flex items-end justify-between gap-4">
              <div>
                <h3 className="text-item-title font-medium text-fg">{group.label}</h3>
                <p className="mt-0.5 text-small-body text-muted">{group.caption}</p>
              </div>
              <Eyebrow className="text-subtle">{group.swatches.length} tokens</Eyebrow>
            </header>
            <div className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4">
              {group.swatches.map(swatch => (
                <TokenCard key={swatch.token} swatch={swatch} />
              ))}
            </div>
          </div>
        ))}
      </div>
    </Section>
  );
}

function TokenCard({ swatch }: { swatch: TokenSwatch }) {
  return (
    <article
      data-testid={`token-${swatch.token}`}
      data-token={swatch.token}
      data-kind={swatch.kind}
      className="flex flex-col gap-3 rounded-lg border border-line bg-canvas-soft p-3"
    >
      <TokenPreview swatch={swatch} />
      <div className="flex flex-col gap-0.5">
        <Eyebrow className="text-subtle">{swatch.token}</Eyebrow>
        <span className="font-mono text-eyebrow text-muted">{swatch.value}</span>
        {swatch.role ? <span className="text-xs text-muted">{swatch.role}</span> : null}
      </div>
    </article>
  );
}

function TokenPreview({ swatch }: { swatch: TokenSwatch }) {
  if (swatch.kind === "color") {
    return (
      <div
        aria-hidden="true"
        className="h-14 w-full rounded-md border border-line"
        style={{ backgroundColor: `var(${swatch.token})` }}
      />
    );
  }
  if (swatch.kind === "radius") {
    return (
      <div
        aria-hidden="true"
        className="flex h-14 w-full items-center justify-center bg-elevated"
        style={{ borderRadius: `var(${swatch.token})` }}
      >
        <span className="font-mono text-eyebrow text-muted">{swatch.value}</span>
      </div>
    );
  }
  return (
    <div
      aria-hidden="true"
      className="flex h-14 w-full items-center justify-center rounded-md bg-elevated"
    >
      <Eyebrow className="text-muted">{swatch.value}</Eyebrow>
    </div>
  );
}

export function TypographySection() {
  return (
    <Section
      id="typography"
      data-testid="section-typography"
      label={<SectionLink section={sectionById("typography")}>Foundations: Typography</SectionLink>}
      right={<Pill mono>Inter · JetBrains Mono · NuixyberNext</Pill>}
    >
      <div className="grid gap-3 pt-4 md:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>Page title · Inter 20/700</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <p className="text-xl font-medium leading-7 tracking-tight" style={{ fontWeight: 510 }}>
              Runtime sessions overview
            </p>
            <p className="text-base leading-7 text-muted">
              Body · Inter 16px regular, the default reading text for operator UI. Line-height
              1.5–1.7 keeps dense dashboards breathable without resorting to oversized padding.
            </p>
            <p className="text-small-body leading-small-body text-subtle">
              Small body · Inter 13px, helper text, captions, meta rows.
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Mono & wordmark</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <Eyebrow className="text-muted">Eyebrow · JetBrains Mono 11/600 0.06em</Eyebrow>
            <p className="font-mono text-sm leading-7 text-fg">agh-network/v0 · run_id_01hq8…</p>
            <div className="flex items-center gap-3">
              <span className="font-wordmark text-display-2xl leading-none tracking-tight text-fg">
                agh
              </span>
              <Pill tone="neutral" size="sm" className="border-line">
                Alpha
              </Pill>
            </div>
          </CardContent>
        </Card>
      </div>
    </Section>
  );
}
