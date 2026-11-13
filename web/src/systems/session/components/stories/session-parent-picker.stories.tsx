import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { sessionFixtures } from "../../mocks/fixtures";
import type { SessionPayload } from "../../types";
import { SessionParentPicker } from "../session-parent-picker";

/** A user session in one project, with the lineage the picker reads. */
function pickerSession(
  index: number,
  lineage: Partial<NonNullable<SessionPayload["lineage"]>> = {}
): SessionPayload {
  const base = sessionFixtures[index % sessionFixtures.length]!;
  return {
    ...base,
    id: `${base.id}-picker`,
    type: "user",
    workspace_id: sessionFixtures[0]!.workspace_id,
    archived_at: null,
    lineage: {
      ...base.lineage!,
      parent_session_id: "",
      root_session_id: `${base.id}-picker`,
      spawn_depth: 0,
      spawn_role: "",
      kind: "",
      ...lineage,
    },
  } as SessionPayload;
}

const SOURCE = pickerSession(0);
const CANDIDATES = [1, 2, 3, 4, 5].map(index => pickerSession(index));

const meta: Meta<typeof SessionParentPicker> = {
  title: "systems/session/components/SessionParentPicker",
  component: SessionParentPicker,
  parameters: { layout: "centered" },
  args: {
    open: true,
    source: SOURCE,
    sourceFailed: false,
    candidates: CANDIDATES,
    search: "",
    loading: false,
    stale: false,
    failed: false,
    hasNext: true,
    hasPrevious: false,
    onNextPage: fn(),
    onPreviousPage: fn(),
    pending: false,
    onOpenChange: fn(),
    onSearchChange: fn(),
    onRetry: fn(),
    onChoose: fn(),
    onMakeTopLevel: fn(),
  },
};
export default meta;
type Story = StoryObj<typeof meta>;

/** Move under…: the project's eligible sessions, searchable, Enter moves under the selected one. */
export const WithResults: Story = {};

/** A forked session keeps its source as its parent: the picker says why, with no parents. */
export const ForkedSession: Story = {
  args: {
    source: pickerSession(0, { parent_session_id: sessionFixtures[1]!.id, kind: "fork" }),
  },
};

/** An adopted child moves back to the top level first. */
export const AdoptedChild: Story = {
  args: {
    source: pickerSession(0, {
      parent_session_id: sessionFixtures[1]!.id,
      root_session_id: sessionFixtures[1]!.id,
      spawn_depth: 1,
      kind: "provenance",
    }),
  },
};

/** No session matches the search. */
export const NoMatches: Story = { args: { search: "zzz", candidates: [] } };

/** The parent read failed: retry in place. */
export const LoadFailed: Story = { args: { failed: true, candidates: [] } };
