import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { SessionDirectiveText } from "@/components/assistant-ui/session-directive-text";

import { SessionMessageCard } from "../session-message-card";
import { SessionReplyCard } from "../session-reply-card";
import { SessionSentCard } from "../session-sent-card";
import {
  ANSWER,
  QUESTION,
  at,
  longTitleCrossWorkspace,
  longTitleTarget,
} from "./session-message-story-fixtures";

const onOpen = fn();

function Text({ children }: { children: string }) {
  return <SessionDirectiveText text={children} directives={[]} />;
}

function Width({ label, className, testId }: { label: string; className: string; testId: string }) {
  return (
    <div className="flex flex-col gap-2">
      <span className="font-mono text-mono-id text-faint">{label}</span>
      <div className={`flex flex-col gap-3 ${className}`} data-testid={testId}>
        <LongTitleCards />
      </div>
    </div>
  );
}

/** #1097: a long title ellipsizes in the middle column; chips, reply state and time stay whole. */
function LongTitleCards() {
  return (
    <>
      <SessionSentCard
        target={longTitleTarget}
        callState="sent"
        mode="steer"
        firstLine="Converse em português brasileiro; artefatos em inglês."
        error={null}
        reply="waiting"
        timestampMs={at(0)}
        onOpenTarget={onOpen}
      />
      <SessionSentCard
        target={{ ...longTitleTarget, title: null }}
        callState="sent"
        mode="queue"
        firstLine={QUESTION}
        error={null}
        reply="completed"
        timestampMs={at(0)}
        onOpenTarget={onOpen}
      />
      <SessionMessageCard
        sender={longTitleCrossWorkspace}
        delivery="steered"
        replyRequested
        timestampMs={at(1)}
        onOpenSender={onOpen}
      >
        <Text>{QUESTION}</Text>
      </SessionMessageCard>
      <SessionMessageCard
        sender={longTitleTarget}
        delivery={null}
        replyRequested={false}
        timestampMs={at(2)}
      >
        <Text>{QUESTION}</Text>
      </SessionMessageCard>
      <SessionReplyCard
        target={longTitleTarget}
        outcome="completed"
        text={ANSWER}
        truncated={false}
        timestampMs={at(3)}
        onOpenTarget={onOpen}
      />
    </>
  );
}

function LongTitleStates() {
  return (
    <div className="flex flex-col gap-8 p-8">
      <Width label="narrow · 360px" className="w-[360px]" testId="long-title-narrow" />
      <Width
        label="wide · 720px column, card at its 560px max"
        className="w-[720px]"
        testId="long-title-wide"
      />
    </div>
  );
}

const meta: Meta = {
  title: "systems/session/components/session-messages/SessionMessages/LongTitles",
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "#1097: a session title longer than the card's middle column ellipsizes on the Sent to, From and Reply from cards; the verb, chips, reply state and time keep their size.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const LongTitles: Story = { render: () => <LongTitleStates /> };
