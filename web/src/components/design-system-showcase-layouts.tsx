import {
  Avatar,
  AvatarFallback,
  Button,
  CodeBlock,
  ListingRow,
  Pill,
  Section,
  ToolCallRow,
} from "@compozy/ui";
import { BoxesIcon } from "lucide-react";

import { SectionLink } from "./design-system-showcase-section-link";
import { sectionById } from "./design-system-showcase-sections";

export function CodeAndChatSection() {
  const sampleCode = `compozy start --workspace compozy-core
compozy session list --active`;

  return (
    <Section
      id="code-chat"
      data-testid="section-code-chat"
      label={<SectionLink section={sectionById("code-chat")}>Code & Chat</SectionLink>}
      right={
        <Pill mono tone="neutral">
          session shells
        </Pill>
      }
    >
      <div className="grid gap-4 pt-4 md:grid-cols-2">
        <CodeBlock code={sampleCode} language="shell" showPrompt />
        <ToolCallRow toolName="read_file" preview="internal/daemon/daemon.go" status="running" />
      </div>
    </Section>
  );
}

export function ListingRowSection() {
  return (
    <Section
      id="listing-row"
      data-testid="section-listing-row"
      label={<SectionLink section={sectionById("listing-row")}>ListingRow</SectionLink>}
      right={<Pill mono>inventory</Pill>}
    >
      <div className="overflow-hidden rounded-lg bg-card shadow-card">
        <ListingRow>
          <ListingRow.Link href="#listing-alpha" aria-label="Open alpha-delivery">
            <ListingRow.Icon>
              <BoxesIcon aria-hidden="true" className="size-4" />
            </ListingRow.Icon>
            <ListingRow.Main>
              <ListingRow.Name>
                <ListingRow.Title>alpha-delivery</ListingRow.Title>
                <Pill size="xs" tone="neutral">
                  Workspace
                </Pill>
                <ListingRow.Slug>v1.2.0</ListingRow.Slug>
              </ListingRow.Name>
              <ListingRow.Description>
                Ships a release candidate through the gate.
              </ListingRow.Description>
              <ListingRow.Meta>
                <span className="font-mono text-micro text-subtle">3 inputs</span>
                <span aria-hidden="true" className="size-0.5 rounded-full bg-faint" />
                <span>iteration cap 8</span>
              </ListingRow.Meta>
            </ListingRow.Main>
          </ListingRow.Link>
          <ListingRow.Trail>
            <Pill mono size="sm" tone="neutral">
              delivery
            </Pill>
            <ListingRow.Stat>
              <ListingRow.Stat.Value>92%</ListingRow.Stat.Value>
              <ListingRow.Stat.Label>12 runs · 30d</ListingRow.Stat.Label>
            </ListingRow.Stat>
            <Button size="sm" type="button" variant="outline">
              Run
            </Button>
          </ListingRow.Trail>
        </ListingRow>
        <ListingRow selected>
          <ListingRow.Link href="#listing-beta" aria-label="Open beta-watch">
            <ListingRow.Icon>
              <Avatar shape="circle" size="sm">
                <AvatarFallback>OP</AvatarFallback>
              </Avatar>
            </ListingRow.Icon>
            <ListingRow.Main>
              <ListingRow.Name>
                <ListingRow.Title>@operator</ListingRow.Title>
                <Pill size="xs" tone="neutral">
                  peer
                </Pill>
              </ListingRow.Name>
              <ListingRow.Description>Selected row uses --selected.</ListingRow.Description>
              <ListingRow.Meta>
                <span>2h ago</span>
              </ListingRow.Meta>
            </ListingRow.Main>
          </ListingRow.Link>
          <ListingRow.Trail>
            <span className="text-eyebrow text-muted">2h</span>
          </ListingRow.Trail>
        </ListingRow>
        <ListingRow>
          <ListingRow.Link href="#listing-gamma" aria-label="Open gamma">
            <ListingRow.Icon>
              <BoxesIcon aria-hidden="true" className="size-4" />
            </ListingRow.Icon>
            <ListingRow.Main>
              <ListingRow.Name mono>
                <ListingRow.Title>sessions/operator.token</ListingRow.Title>
              </ListingRow.Name>
              <ListingRow.Meta>
                <span>updated 2h ago</span>
              </ListingRow.Meta>
            </ListingRow.Main>
          </ListingRow.Link>
          <ListingRow.Trail>
            <Pill mono size="sm" tone="neutral">
              sessions
            </Pill>
          </ListingRow.Trail>
        </ListingRow>
      </div>
    </Section>
  );
}
