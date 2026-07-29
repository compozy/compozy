import type { Meta, StoryObj } from "@storybook/react-vite";
import { ChevronUpIcon } from "lucide-react";

import { Button } from "../../button";
import { Dock } from "../dock";

const meta: Meta<typeof Dock> = {
  title: "components/custom/Dock",
  component: Dock,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Decision panel fused to the composer top — permissions and clarifications leave the transcript and dock here. Open bottom edge (the composer below closes the shape), eyebrow + title head, mono subject on the code wash, actions with keyboard chips. No tinted washes; emphasis is geometry.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Permission: Story = {
  render: () => (
    <div className="max-w-xl">
      <Dock>
        <Dock.Head>
          <Dock.Eyebrow>Permission</Dock.Eyebrow>
          <Dock.Title>Run a command</Dock.Title>
          <Dock.Count>1/1</Dock.Count>
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
          <Button size="sm" variant="outline">
            Always allow
            <Dock.Key>2</Dock.Key>
          </Button>
          <span className="flex-1" />
          <Button size="sm" variant="ghost">
            Reject
            <Dock.Key>3</Dock.Key>
          </Button>
          <Button size="sm" variant="ghost" aria-label="More reject options">
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
