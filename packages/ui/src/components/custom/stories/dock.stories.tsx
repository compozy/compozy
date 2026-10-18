import type { Meta, StoryObj } from "@storybook/react-vite";
import { ChevronUpIcon, Database, SquareTerminal } from "lucide-react";

import { Button } from "../../button";
import { Dock } from "../dock";
import { KindIcon } from "../kind-icon";

const meta: Meta<typeof Dock> = {
  title: "components/custom/Dock",
  component: Dock,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          'Decision card above the composer — permissions and clarifications leave the transcript and dock here. Approval-card anatomy: a standalone `canvas` card with `shadow-card`, an identity-well head (`KindIcon tone="well"`) with a 15px title, the subject recessed on `sunken`, and a pill action row with one inverted primary. No tinted washes.',
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** The prototype approval card: well + title, sunken command, hint + Deny + inverted "Allow once ↵". */
export const Permission: Story = {
  render: () => (
    <div className="max-w-xl">
      <Dock>
        <Dock.Head>
          <KindIcon icon={SquareTerminal} tone="well" />
          <Dock.Title>claude wants to run a command</Dock.Title>
          <Dock.Count>1/1</Dock.Count>
        </Dock.Head>
        <Dock.Pre>bun run db:migrate --name plans_v2</Dock.Pre>
        <Dock.Actions>
          <span className="mr-auto inline-flex items-center gap-2 text-small-body text-muted">
            <Database aria-hidden="true" className="size-3.75" />
            Writes to the local database
          </span>
          <Button variant="neutral">Deny</Button>
          <Button variant="primary" kbd="↵" aria-keyshortcuts="Enter">
            Allow once
          </Button>
        </Dock.Actions>
      </Dock>
    </div>
  ),
};

/** Stacked decision with an eyebrow, meta line and keyboard chips on every action. */
export const PermissionWithKeys: Story = {
  render: () => (
    <div className="max-w-xl">
      <Dock>
        <Dock.Head>
          <KindIcon icon={SquareTerminal} tone="well" />
          <Dock.Eyebrow>Permission</Dock.Eyebrow>
          <Dock.Title>Run a command</Dock.Title>
          <Dock.Count>1/2</Dock.Count>
        </Dock.Head>
        <Dock.Body>
          <Dock.Pre>bunx turbo run lint typecheck test --filter=./web</Dock.Pre>
          <Dock.Meta>
            Bash · workspace <code>compozy</code>
          </Dock.Meta>
        </Dock.Body>
        <Dock.Actions>
          <Button size="sm">
            Allow once
            <Dock.Key>1</Dock.Key>
          </Button>
          <Button size="sm" variant="neutral">
            Always allow
            <Dock.Key>2</Dock.Key>
          </Button>
          <span className="flex-1" />
          <Button size="sm" variant="quiet">
            Reject
            <Dock.Key>3</Dock.Key>
          </Button>
          <Button size="sm" variant="quiet" aria-label="More reject options">
            <ChevronUpIcon />
          </Button>
        </Dock.Actions>
      </Dock>
    </div>
  ),
};

export const WithDeadline: Story = {
  parameters: {
    docs: {
      description: {
        story: "The deadline hint is static mono — it never ticks; the broker enforces timeouts.",
      },
    },
  },
  render: () => (
    <div className="max-w-xl">
      <Dock>
        <Dock.Head>
          <Dock.Eyebrow>Question</Dock.Eyebrow>
          <Dock.Title>Where should the marker stories live?</Dock.Title>
          <Dock.Deadline>times out 14:32</Dock.Deadline>
        </Dock.Head>
      </Dock>
    </div>
  ),
};

export const StatusLines: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "Submitting and retryable-error states are quiet status lines under the actions — never an Alert card.",
      },
    },
  },
  render: () => (
    <div className="flex max-w-xl flex-col gap-4">
      <Dock>
        <Dock.Head>
          <Dock.Eyebrow>Question</Dock.Eyebrow>
          <Dock.Title>Sending your answer</Dock.Title>
        </Dock.Head>
        <Dock.Status>Sending answer…</Dock.Status>
      </Dock>
      <Dock>
        <Dock.Head>
          <Dock.Eyebrow>Question</Dock.Eyebrow>
          <Dock.Title>Where should the marker stories live?</Dock.Title>
        </Dock.Head>
        <Dock.Status tone="danger">Could not send the answer — try again.</Dock.Status>
      </Dock>
    </div>
  ),
};
