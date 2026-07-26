import {
  Avatar,
  AvatarFallback,
  Button,
  ChatMessageBubble,
  CodeBlock,
  ConnectionIndicator,
  Item,
  ItemContent,
  ItemDescription,
  ItemMedia,
  ItemTitle,
  ListingRow,
  Pill,
  ScrollArea,
  Section,
  Sidebar,
  SplitPane,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  ToolCallRow,
} from "@agh/ui";
import {
  BellIcon,
  BoxesIcon,
  FolderIcon,
  HomeIcon,
  SettingsIcon,
  SquareTerminalIcon,
  WaypointsIcon,
} from "lucide-react";
import type { ComponentType } from "react";

import { SectionLink } from "./design-system-showcase-section-link";
import { sectionById } from "./design-system-showcase-sections";

export function CodeAndChatSection() {
  const sampleCode = `agh start --workspace agh-core
agh session list --active`;

  return (
    <Section
      id="code-chat"
      data-testid="section-code-chat"
      label={<SectionLink section={sectionById("code-chat")}>Code & Chat</SectionLink>}
      right={
        <Pill mono tone="accent">
          session shells
        </Pill>
      }
    >
      <div className="grid gap-4 pt-4 md:grid-cols-2">
        <CodeBlock code={sampleCode} language="shell" />
        <div className="flex flex-col gap-3">
          <ChatMessageBubble messageRole="user" meta={<span>YOU · 10:42</span>}>
            Spin up a new run against the research workspace.
          </ChatMessageBubble>
          <ChatMessageBubble
            messageRole="agent"
            meta={
              <>
                <Pill.Dot tone="success" size="sm" />
                <span>CLAUDE · 10:42</span>
              </>
            }
          >
            Starting run_01HQ8… against agh-core. Streaming events to the inspector.
          </ChatMessageBubble>
          <ChatMessageBubble messageRole="tool">
            <ToolCallRow
              toolName="read_file"
              preview="internal/daemon/daemon.go"
              status="running"
            />
          </ChatMessageBubble>
          <ChatMessageBubble messageRole="system">Session idle · 2m</ChatMessageBubble>
        </div>
      </div>
    </Section>
  );
}

export function LayoutSection() {
  return (
    <Section
      id="layout"
      data-testid="section-layout"
      label={<SectionLink section={sectionById("layout")}>Sidebar & SplitPane</SectionLink>}
      right={<Pill mono>layout</Pill>}
    >
      <div className="grid gap-4 pt-4 lg:grid-cols-2">
        <div className="h-[340px] overflow-hidden rounded-lg border border-line">
          <Sidebar
            defaultCollapsed={false}
            rail={
              <>
                <button
                  type="button"
                  aria-label="Workspace agh-core"
                  className="inline-flex size-7 items-center justify-center rounded-full border border-accent bg-elevated font-mono text-eyebrow text-accent"
                >
                  A
                </button>
                <button
                  type="button"
                  aria-label="Workspace research"
                  className="inline-flex size-7 items-center justify-center rounded-full border border-line bg-canvas-soft font-mono text-eyebrow text-muted"
                >
                  R
                </button>
              </>
            }
            header={
              <>
                <FolderIcon className="size-3 text-subtle" />
                <span className="text-small-body font-medium">agh-core</span>
              </>
            }
            nav={
              <div className="flex flex-col gap-0.5 px-2 py-3 text-sm">
                <SidebarRow icon={HomeIcon} label="Home" active />
                <SidebarRow icon={SquareTerminalIcon} label="Sessions" />
                <SidebarRow icon={BoxesIcon} label="Tasks" />
                <SidebarRow icon={WaypointsIcon} label="Network" />
                <SidebarRow icon={BellIcon} label="Automation" />
              </div>
            }
            footer={
              <div className="flex items-center justify-between gap-2 text-xs text-subtle">
                <ConnectionIndicator status="connected" />
                <Button variant="ghost" size="icon-sm" aria-label="Settings">
                  <SettingsIcon className="size-3" />
                </Button>
              </div>
            }
          />
        </div>
        <div className="h-[340px] overflow-hidden rounded-lg border border-line">
          <SplitPane
            list={
              <ScrollArea className="h-full">
                <ul className="flex flex-col divide-y divide-line">
                  {[
                    "Skill: repo-refactor",
                    "Skill: ship-review",
                    "Skill: rebase-helper",
                    "Skill: test-writer",
                  ].map((entry, index) => (
                    <li key={entry}>
                      <Item data-active={index === 0 ? "true" : undefined}>
                        <ItemMedia>
                          <Avatar className="size-7">
                            <AvatarFallback>{entry.slice(7, 8).toUpperCase()}</AvatarFallback>
                          </Avatar>
                        </ItemMedia>
                        <ItemContent>
                          <ItemTitle>{entry}</ItemTitle>
                          <ItemDescription>Updated 2m ago</ItemDescription>
                        </ItemContent>
                      </Item>
                    </li>
                  ))}
                </ul>
              </ScrollArea>
            }
            detail={
              <div className="flex flex-1 flex-col gap-4 p-4">
                <nav
                  aria-label="Breadcrumb"
                  className="flex items-center gap-1.5 text-small-body text-muted"
                >
                  <a href="#" className="transition-colors duration-base ease-out hover:text-fg">
                    Skills
                  </a>
                  <span aria-hidden="true" className="text-faint">
                    /
                  </span>
                  <span aria-current="page" className="text-fg">
                    repo-refactor
                  </span>
                </nav>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Key</TableHead>
                      <TableHead>Value</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <TableRow>
                      <TableCell>Version</TableCell>
                      <TableCell>
                        <Pill mono tone="accent">
                          v0.4.2
                        </Pill>
                      </TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell>Last run</TableCell>
                      <TableCell>2h ago</TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </div>
            }
          />
        </div>
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
      <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
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
                <span className="font-mono text-[10px] text-subtle">3 inputs</span>
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
              <ListingRow.Description>Selected row uses --row-selected.</ListingRow.Description>
              <ListingRow.Meta>
                <span>2h ago</span>
              </ListingRow.Meta>
            </ListingRow.Main>
          </ListingRow.Link>
          <ListingRow.Trail>
            <span className="text-[11px] text-faint">2h</span>
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

function SidebarRow({
  icon: Icon,
  label,
  active,
}: {
  icon: ComponentType<{ className?: string }>;
  label: string;
  active?: boolean;
}) {
  return (
    <button
      type="button"
      data-active={active ? "true" : undefined}
      className="group flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-small-body text-muted transition-colors hover:bg-hover hover:text-fg data-[active=true]:bg-elevated data-[active=true]:text-fg"
    >
      <Icon className="size-3 text-subtle group-data-[active=true]:text-accent" />
      <span>{label}</span>
    </button>
  );
}
