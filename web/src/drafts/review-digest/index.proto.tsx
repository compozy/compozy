/**
 * Draft: a "Ready for you" digest above the Sessions board that gathers the pull requests waiting
 * on a human decision. Free markup is allowed here; once the direction is approved it becomes a
 * view in `systems/session` and this slug moves there as a spec prototype.
 */
import { CheckIcon, GitPullRequestIcon, XIcon } from "@compozy/ui/icons";

import { Button, Card, CardContent, Eyebrow, Pill } from "@compozy/ui";

import { definePrototype, type PrototypeProps } from "@compozy/proto";

export const meta = definePrototype({
  title: "Review digest",
  spec: "sessions-kanban",
  route: "/sessions?view=board",
  states: [
    { id: "digest", label: "Three pull requests waiting" },
    { id: "clear", label: "Nothing waiting" },
  ],
});

type Waiting = {
  number: number;
  title: string;
  verdict: string;
  tone: "success" | "danger" | "neutral";
};

const waiting: Waiting[] = [
  {
    number: 931,
    title: "Topbar PR chip — web slice",
    verdict: "Approved",
    tone: "success",
  },
  {
    number: 932,
    title: "Board column SQL expression",
    verdict: "2 checks failing",
    tone: "danger",
  },
  {
    number: 919,
    title: "Merge train: #919–#923",
    verdict: "Review requested",
    tone: "neutral",
  },
];

const actionLabel: Record<Waiting["tone"], string> = {
  success: "Merge",
  danger: "Fix checks",
  neutral: "Review",
};

function Row({ item }: { item: Waiting }) {
  return (
    <li className="flex items-center gap-3 rounded-md px-3 py-2 hover:bg-hover">
      <GitPullRequestIcon aria-hidden="true" className="size-4 shrink-0 text-subtle" />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-small-body font-medium text-fg">{item.title}</span>
        <span className="font-mono text-mono-id tracking-mono-id text-subtle">#{item.number}</span>
      </span>
      <Pill tone={item.tone}>
        {item.tone === "success" ? <CheckIcon aria-hidden="true" /> : null}
        {item.tone === "danger" ? <XIcon aria-hidden="true" /> : null}
        {item.verdict}
      </Pill>
      <Button size="sm" variant={item.tone === "success" ? "primary" : "secondary"}>
        {actionLabel[item.tone]}
      </Button>
    </li>
  );
}

export default function ReviewDigestDraft({ state }: PrototypeProps) {
  const items = state === "clear" ? [] : waiting;
  return (
    <main className="flex min-h-dvh items-start justify-center bg-canvas p-8 text-fg">
      <Card className="w-full max-w-xl">
        <CardContent className="flex flex-col gap-3">
          <header className="flex items-baseline justify-between gap-3">
            <div className="flex flex-col gap-1">
              <Eyebrow variant="caps">Ready for you</Eyebrow>
              <h1 className="text-item-title font-medium">
                {items.length > 0
                  ? `${items.length} pull requests need a decision`
                  : "All caught up"}
              </h1>
            </div>
            <Button size="sm" variant="quiet">
              Open board
            </Button>
          </header>
          {items.length > 0 ? (
            <ul className="-mx-3 flex flex-col">
              {items.map(item => (
                <Row item={item} key={item.number} />
              ))}
            </ul>
          ) : (
            <p className="text-small-body text-muted">
              No pull request is waiting on you. New reviews show up here as agents open them.
            </p>
          )}
        </CardContent>
      </Card>
    </main>
  );
}
