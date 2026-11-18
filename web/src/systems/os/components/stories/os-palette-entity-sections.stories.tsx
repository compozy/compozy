import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";
import { Command, CommandInput, CommandList } from "@compozy/ui";

import type { OsPaletteEntities } from "../../hooks/use-os-palette-entities";
import { OsPaletteEntitySections } from "../os-palette-entity-sections";

const FOCUSED_SESSION = "session:focused";

const ENTITIES: OsPaletteEntities = {
  sessions: [],
  sessionTotal: 0,
  tabs: [],
  tabTotal: 0,
  worktrees: [],
  worktreeTotal: 0,
  selectWorktree: fn(),
  linkSessionId: null,
  moveSessionId: FOCUSED_SESSION,
};

/** The palette's entity rows inside its own Command frame, with the query that reaches them. */
function PaletteFrame({ query, entities }: { query: string; entities: OsPaletteEntities }) {
  return (
    <Command className="w-140 rounded-lg border border-line shadow-pop" shouldFilter={false}>
      <CommandInput readOnly value={query} variant="quiet" />
      <CommandList>
        <OsPaletteEntitySections
          destination={false}
          entities={entities}
          onGoToTab={fn()}
          onLinkSession={fn()}
          onMoveSession={fn()}
          onOpenSession={fn()}
          onSelectWorktree={fn()}
        />
      </CommandList>
    </Command>
  );
}

const meta: Meta<typeof PaletteFrame> = {
  title: "systems/os/components/OsPaletteEntitySections",
  component: PaletteFrame,
  parameters: { layout: "centered" },
  args: { query: "move", entities: ENTITIES },
};
export default meta;
type Story = StoryObj<typeof meta>;

/** "move" or "parent" with a session window focused reaches its Move under… command. */
export const ThisSessionMoveUnder: Story = {};

/** Both focused-session commands when the query reaches each. */
export const ThisSessionCommands: Story = {
  args: { query: "session", entities: { ...ENTITIES, linkSessionId: FOCUSED_SESSION } },
};
