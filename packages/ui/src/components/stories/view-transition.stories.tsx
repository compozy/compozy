import type { Meta, StoryObj } from "@storybook/react-vite";
import { startTransition, Suspense, use, useState, ViewTransition } from "react";

import { BoxesIcon } from "../../icons";
import {
  startTypedTransition,
  VIEW_TRANSITION_CLASS,
  VIEW_TRANSITION_DRILL,
  VIEW_TRANSITION_TYPE,
} from "../../lib/view-transition";
import { Button } from "../button";
import { ListingRowSkeleton } from "../custom/content-skeleton";
import { ListingRow } from "../custom/listing-row";
import { SkeletonRegion } from "../skeleton";

const meta: Meta = {
  title: "components/ui/ViewTransition",
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          'React\'s `<ViewTransition>` with the in-window vocabulary from `tokens-runtime.css`: `VIEW_TRANSITION_CLASS` names the classes, `startTypedTransition` tags a change with a cause, and `VIEW_TRANSITION_DRILL` picks the drill class by that cause. A boundary animates only for updates React treats as transitions (`startTransition`, a Suspense reveal, `useDeferredValue`), so every story below changes state inside one. Start every boundary from `default="none"` and opt in to the triggers it needs, so unrelated transitions (a router navigation, any other `startTransition`) never snapshot its subtree; never wrap high-frequency surfaces (streaming transcript, terminal, drag and resize, window geometry). Motion is opacity and transform from the motion tokens, and reduced motion cuts it.',
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const PANEL = "w-[420px] overflow-hidden rounded-lg bg-card shadow-card";

function FadeDemo() {
  const [tab, setTab] = useState<"overview" | "activity">("overview");

  return (
    <div className="flex w-[420px] flex-col gap-3">
      <div className="flex gap-1">
        {(["overview", "activity"] as const).map(name => (
          <Button
            key={name}
            size="sm"
            type="button"
            variant={tab === name ? "secondary" : "ghost"}
            onClick={() => startTransition(() => setTab(name))}
          >
            {name}
          </Button>
        ))}
      </div>
      <ViewTransition default="none" update={VIEW_TRANSITION_CLASS.fade}>
        <div className={PANEL}>
          {tab === "overview" ? (
            <p className="p-4 text-small-body text-muted-foreground">
              Overview: the panel cross-fades as its content changes inside a transition.
            </p>
          ) : (
            <ul className="flex flex-col gap-1 p-4 text-small-body text-muted-foreground">
              <li>Activity: session started</li>
              <li>Activity: tool call approved</li>
              <li>Activity: session finished</li>
            </ul>
          )}
        </div>
      </ViewTransition>
    </div>
  );
}

/** `update={VIEW_TRANSITION_CLASS.fade}`: segment and tab panels swap with a cross-fade. */
export const Fade: Story = {
  render: () => <FadeDemo />,
};

function DrillDemo() {
  const [view, setView] = useState<"list" | "detail">("list");

  return (
    <div className={PANEL}>
      <ViewTransition
        key={view}
        default="none"
        enter={VIEW_TRANSITION_DRILL}
        exit={VIEW_TRANSITION_DRILL}
      >
        {view === "list" ? (
          <div className="flex flex-col items-start gap-3 p-4">
            <p className="text-small-body text-muted-foreground">
              A list. Opening a row drills in.
            </p>
            <Button
              size="sm"
              type="button"
              onClick={() =>
                startTypedTransition(VIEW_TRANSITION_TYPE.drillIn, () => setView("detail"))
              }
            >
              Open detail
            </Button>
          </div>
        ) : (
          <div className="flex flex-col items-start gap-3 p-4">
            <p className="text-small-body text-muted-foreground">
              The detail. Going back drills out.
            </p>
            <Button
              size="sm"
              type="button"
              variant="outline"
              onClick={() =>
                startTypedTransition(VIEW_TRANSITION_TYPE.drillOut, () => setView("list"))
              }
            >
              Back to list
            </Button>
          </div>
        )}
      </ViewTransition>
    </div>
  );
}

/**
 * A keyed boundary swaps a list and its detail. `startTypedTransition` tags the change with
 * `drill-in` or `drill-out`, and `VIEW_TRANSITION_DRILL` turns that into the matching class.
 */
export const Drill: Story = {
  render: () => <DrillDemo />,
};

function ItemsDemo() {
  const [items, setItems] = useState(["Alpha", "Beta", "Gamma"]);
  const [serial, setSerial] = useState(4);

  return (
    <div className="flex w-[420px] flex-col gap-3">
      <Button
        className="self-start"
        size="sm"
        type="button"
        onClick={() =>
          startTransition(() => {
            setItems(current => [...current, `Item ${serial}`]);
            setSerial(current => current + 1);
          })
        }
      >
        Add item
      </Button>
      <ul className={PANEL}>
        {items.map(name => (
          <ViewTransition
            key={name}
            default="none"
            enter={VIEW_TRANSITION_CLASS.itemEnter}
            exit={VIEW_TRANSITION_CLASS.itemExit}
          >
            <li className="flex items-center justify-between gap-3 border-b border-line-soft px-4 py-2 text-small-body last:border-b-0">
              {name}
              <Button
                size="xs"
                type="button"
                variant="ghost"
                onClick={() =>
                  startTransition(() => setItems(current => current.filter(item => item !== name)))
                }
              >
                Remove
              </Button>
            </li>
          </ViewTransition>
        ))}
      </ul>
    </div>
  );
}

/** `enter` and `exit` on each keyed item: it rises into place and sinks away. */
export const Items: Story = {
  render: () => <ItemsDemo />,
};

interface LoadState {
  id: number;
  rows: Promise<readonly string[]>;
}

function loadRows(): Promise<readonly string[]> {
  return new Promise(resolve => {
    setTimeout(() => resolve(["alpha-delivery", "beta-watch", "gamma-audit"]), 900);
  });
}

function LoadedRows({ rows }: { rows: Promise<readonly string[]> }) {
  const names = use(rows);
  return (
    <div className={PANEL}>
      {names.map(name => (
        <ListingRow key={name} interactive={false}>
          <ListingRow.Icon>
            <BoxesIcon aria-hidden="true" className="size-4" />
          </ListingRow.Icon>
          <ListingRow.Main>
            <ListingRow.Name>
              <ListingRow.Title>{name}</ListingRow.Title>
            </ListingRow.Name>
            <ListingRow.Meta>
              <span>3 inputs</span>
              <span>iteration cap 8</span>
            </ListingRow.Meta>
          </ListingRow.Main>
          <ListingRow.Trail>
            <ListingRow.Stat>
              <ListingRow.Stat.Value>92%</ListingRow.Stat.Value>
              <ListingRow.Stat.Label>12 runs · 30d</ListingRow.Stat.Label>
            </ListingRow.Stat>
          </ListingRow.Trail>
        </ListingRow>
      ))}
    </div>
  );
}

function SuspenseDemo() {
  const [load, setLoad] = useState<LoadState>(() => ({ id: 0, rows: loadRows() }));

  return (
    <div className="flex w-[420px] flex-col gap-3">
      <Button
        className="self-start"
        size="sm"
        type="button"
        onClick={() => {
          // Start the read outside the updater: React may call updaters twice.
          const rows = loadRows();
          startTransition(() => setLoad(current => ({ id: current.id + 1, rows })));
        }}
      >
        Reload
      </Button>
      <ViewTransition default="none" update={VIEW_TRANSITION_CLASS.fade}>
        <Suspense
          key={load.id}
          fallback={
            <SkeletonRegion className={PANEL} label="Loading rows">
              <ListingRowSkeleton count={3} description={false} />
            </SkeletonRegion>
          }
        >
          <LoadedRows rows={load.rows} />
        </Suspense>
      </ViewTransition>
    </div>
  );
}

/**
 * A Suspense boundary inside `<ViewTransition update=…>`: the content-shaped skeleton
 * cross-fades into the loaded rows, which have the same height, so nothing moves.
 */
export const SuspenseReveal: Story = {
  render: () => <SuspenseDemo />,
};
