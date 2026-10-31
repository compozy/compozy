import type { ReactNode } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { UserMessageBubble } from "@/components/assistant-ui/session-user-message";
import { SessionDirectiveText } from "@/components/assistant-ui/session-directive-text";
import type { SessionAttachmentItem } from "@/systems/session";

import { MessageMarkdown } from "../../message-markdown";
import { SessionAttachmentGallery } from "../../session-attachment-gallery";
import { SessionMessageCard } from "../session-message-card";
import { SessionReplyCard } from "../session-reply-card";
import { SessionSentCard } from "../session-sent-card";
import {
  ANSWER,
  LONG_MESSAGE,
  QUESTION,
  at,
  billingReviewer,
  crossWorkspaceSender,
  deletedSender,
  deletedTarget,
  refactorBilling,
  renamedSender,
} from "./session-message-story-fixtures";

const onOpen = fn();

function Cell({ label, children, wide }: { label: string; children: ReactNode; wide?: boolean }) {
  return (
    <div className={wide ? "col-span-2 flex flex-col gap-2" : "flex flex-col gap-2"}>
      <span className="font-mono text-mono-id text-faint">{label}</span>
      {children}
    </div>
  );
}

function Text({ children }: { children: string }) {
  return <SessionDirectiveText text={children} directives={[]} />;
}

const attachments: SessionAttachmentItem[] = [
  {
    kind: "file",
    id: "att-1",
    href: "#att-1",
    filename: "retry-budget.md",
    extension: "MD",
    sizeLabel: "4.1 KB",
  },
  {
    kind: "file",
    id: "att-2",
    href: "#att-2",
    filename: "billing.toml",
    extension: "TOML",
    sizeLabel: "812 B",
  },
];

/** VC-01: the session message card (S1) in every state `_uiux.md` lists. */
function MessageCardStates() {
  return (
    <div className="grid max-w-6xl grid-cols-2 gap-x-6 gap-y-5 p-8">
      <Cell label="default · direct or from the queue · no chips">
        <SessionMessageCard
          sender={refactorBilling}
          delivery={null}
          replyRequested={false}
          timestampMs={at(0)}
          onOpenSender={onOpen}
        >
          <Text>{QUESTION}</Text>
        </SessionMessageCard>
      </Cell>
      <Cell label="reply requested · US-006.AC-3">
        <SessionMessageCard
          sender={refactorBilling}
          delivery={null}
          replyRequested
          timestampMs={at(0)}
          onOpenSender={onOpen}
        >
          <Text>{QUESTION}</Text>
        </SessionMessageCard>
      </Cell>
      <Cell label="steered · US-008.AC-2">
        <SessionMessageCard
          sender={refactorBilling}
          delivery="steered"
          replyRequested
          timestampMs={at(4)}
          onOpenSender={onOpen}
        >
          <Text>Also check whether webhook retries share that budget.</Text>
        </SessionMessageCard>
      </Cell>
      <Cell label="interrupted · interrupt-and-replace">
        <SessionMessageCard
          sender={refactorBilling}
          delivery="interrupted"
          replyRequested
          timestampMs={at(6)}
          onOpenSender={onOpen}
        >
          <Text>Stop the audit. The budget question is all I need.</Text>
        </SessionMessageCard>
      </Cell>
      <Cell label="superseded · subdued, never removed">
        <SessionMessageCard
          sender={refactorBilling}
          delivery="superseded"
          replyRequested={false}
          timestampMs={at(5)}
          onOpenSender={onOpen}
        >
          <Text>Check the per-request path first.</Text>
        </SessionMessageCard>
      </Cell>
      <Cell label="sender deleted · no link · US-006.EC-1">
        <SessionMessageCard
          sender={deletedSender}
          delivery={null}
          replyRequested={false}
          timestampMs={at(-1440)}
          onOpenSender={onOpen}
        >
          <Text>{QUESTION}</Text>
        </SessionMessageCard>
      </Cell>
      <Cell label="sender renamed · current title · US-006.EC-2">
        <SessionMessageCard
          sender={renamedSender}
          delivery={null}
          replyRequested={false}
          timestampMs={at(0)}
          onOpenSender={onOpen}
        >
          <Text>{QUESTION}</Text>
        </SessionMessageCard>
      </Cell>
      <Cell label="cross-workspace sender · US-006.EC-4">
        <SessionMessageCard
          sender={crossWorkspaceSender}
          delivery={null}
          replyRequested={false}
          timestampMs={at(2)}
          onOpenSender={onOpen}
        >
          <Text>Does the billing doc page still describe per-request retries?</Text>
        </SessionMessageCard>
      </Cell>
      <Cell label="long · clamped at 176px · Show more · US-006.EC-3">
        <SessionMessageCard
          sender={refactorBilling}
          delivery={null}
          replyRequested
          timestampMs={at(8)}
          onOpenSender={onOpen}
        >
          <Text>{LONG_MESSAGE}</Text>
        </SessionMessageCard>
      </Cell>
      <Cell label="attachments · reused from UserMessage">
        <SessionMessageCard
          sender={refactorBilling}
          delivery={null}
          replyRequested
          timestampMs={at(9)}
          onOpenSender={onOpen}
          attachments={<SessionAttachmentGallery items={attachments} className="mb-0 self-start" />}
        >
          <Text>Review both files and tell me which limit wins.</Text>
        </SessionMessageCard>
      </Cell>
    </div>
  );
}

/** VC-02: the receiver transcript rhythm — operator bubbles right, assistant text plain, the card left. */
function ReceiverRhythm() {
  return (
    <div className="mx-auto flex max-w-180 flex-col p-8">
      <div className="flex justify-end pb-transcript-turn-gap">
        <div className="flex max-w-[80%] flex-col items-end">
          <UserMessageBubble>
            <Text>
              Read billing.toml and the retry middleware. Stay around — other sessions will ask you
              about billing config.
            </Text>
          </UserMessageBubble>
        </div>
      </div>
      <div className="pb-transcript-turn-gap">
        <MessageMarkdown content="Read both. Retries are configured per job in `billing.toml` and enforced in `internal/billing/retry.go`. Ready for questions." />
      </div>
      <div className="flex pb-transcript-turn-gap">
        <SessionMessageCard
          sender={refactorBilling}
          delivery={null}
          replyRequested
          timestampMs={at(0)}
          onOpenSender={onOpen}
        >
          <Text>{QUESTION}</Text>
        </SessionMessageCard>
      </div>
      <div className="pb-transcript-turn-gap">
        <MessageMarkdown content={ANSWER} />
      </div>
      <div className="flex justify-end">
        <div className="flex max-w-[80%] flex-col items-end">
          <UserMessageBubble>
            <Text>Good. Note that down in the review doc too.</Text>
          </UserMessageBubble>
        </div>
      </div>
    </div>
  );
}

/** VC-03: the reply card (S2) per outcome. */
function ReplyCardStates() {
  return (
    <div className="grid max-w-6xl grid-cols-2 gap-x-6 gap-y-5 p-8">
      <Cell label="completed · US-011.AC-4">
        <SessionReplyCard
          target={billingReviewer}
          outcome="completed"
          text={ANSWER}
          truncated={false}
          timestampMs={at(1)}
          onOpenTarget={onOpen}
        />
      </Cell>
      <Cell label="completed, no text · US-011.EC-1">
        <SessionReplyCard
          target={billingReviewer}
          outcome="completed"
          text=""
          truncated={false}
          timestampMs={at(1)}
          onOpenTarget={onOpen}
        />
      </Cell>
      <Cell label="truncated · reply_truncated · US-011.AC-2" wide>
        <SessionReplyCard
          target={billingReviewer}
          outcome="completed"
          text={LONG_MESSAGE}
          truncated
          timestampMs={at(22)}
          onOpenTarget={onOpen}
        />
      </Cell>
      <Cell label="failed · error summary · US-012.AC-1">
        <SessionReplyCard
          target={billingReviewer}
          outcome="failed"
          text="Provider error: rate limit exceeded (429). Try again later."
          truncated={false}
          timestampMs={at(3)}
          onOpenTarget={onOpen}
        />
      </Cell>
      <Cell label="canceled · US-012.AC-2">
        <SessionReplyCard
          target={billingReviewer}
          outcome="canceled"
          text=""
          truncated={false}
          timestampMs={at(3)}
          onOpenTarget={onOpen}
        />
      </Cell>
      <Cell label="dropped · removed from the queue · US-012.AC-3">
        <SessionReplyCard
          target={billingReviewer}
          outcome="dropped"
          text=""
          truncated={false}
          timestampMs={at(4)}
          onOpenTarget={onOpen}
        />
      </Cell>
      <Cell label="unknown · dispatch uncertain · US-012.EC-4">
        <SessionReplyCard
          target={billingReviewer}
          outcome="unknown"
          text=""
          truncated={false}
          timestampMs={at(10)}
          onOpenTarget={onOpen}
        />
      </Cell>
      <Cell label="target deleted · no link">
        <SessionReplyCard
          target={deletedTarget}
          outcome="completed"
          text="Per job. `retry_budget` caps total attempts across all of a job's requests."
          truncated={false}
          timestampMs={at(-1440)}
          onOpenTarget={onOpen}
        />
      </Cell>
    </div>
  );
}

/** VC-04: the sent card (S3) from the call in flight to the reply's outcome. */
function SentCardStates() {
  const base = {
    target: billingReviewer,
    mode: "queue" as const,
    firstLine: QUESTION,
    error: null,
    timestampMs: at(0),
    onOpenTarget: onOpen,
  };
  return (
    <div className="grid max-w-6xl grid-cols-2 gap-x-6 gap-y-5 p-8">
      <Cell label="sending · call in flight">
        <SessionSentCard {...base} callState="sending" reply="waiting" />
      </Cell>
      <Cell label="sent · no reply requested">
        <SessionSentCard
          {...base}
          callState="sent"
          reply="none"
          firstLine="FYI: I moved the billing client to internal/billing/client."
        />
      </Cell>
      <Cell label="waiting for reply · success pulse">
        <SessionSentCard {...base} callState="sent" reply="waiting" />
      </Cell>
      <Cell label="replied · matched wake_event_id">
        <SessionSentCard {...base} callState="sent" reply="completed" />
      </Cell>
      <Cell label="steered · waiting">
        <SessionSentCard
          {...base}
          callState="sent"
          mode="steer"
          reply="waiting"
          firstLine="Also check whether webhook retries share that budget."
          timestampMs={at(4)}
        />
      </Cell>
      <Cell label="reply failed · danger">
        <SessionSentCard {...base} callState="sent" reply="failed" />
      </Cell>
      <Cell label="reply canceled · interrupted">
        <SessionSentCard
          {...base}
          callState="sent"
          mode="interrupt"
          reply="canceled"
          firstLine="Stop the audit. The budget question is all I need."
          timestampMs={at(6)}
        />
      </Cell>
      <Cell label="reply dropped · operator canceled the queued row">
        <SessionSentCard {...base} callState="sent" reply="dropped" />
      </Cell>
      <Cell label="call failed · US-015.AC-3">
        <SessionSentCard
          {...base}
          callState="failed"
          reply="none"
          error="Message chain limit reached (8 hops). Ask the operator to continue."
        />
      </Cell>
      <Cell label="target deleted · no link">
        <SessionSentCard
          {...base}
          target={deletedTarget}
          callState="sent"
          reply="completed"
          timestampMs={at(-1440)}
        />
      </Cell>
    </div>
  );
}

/** VC-04 · VC-03: the sender transcript once the answer is back. */
function SenderFlow() {
  return (
    <div className="mx-auto flex max-w-180 flex-col gap-3 p-8">
      <MessageMarkdown content="I'll ask the reviewer session and wait for its answer before touching retries." />
      <SessionSentCard
        target={billingReviewer}
        callState="sent"
        mode="queue"
        firstLine={QUESTION}
        error={null}
        reply="completed"
        timestampMs={at(0)}
        onOpenTarget={onOpen}
      />
      <MessageMarkdown content="Asked. I'll end the turn here and pick up when the answer arrives." />
      <SessionReplyCard
        target={billingReviewer}
        outcome="completed"
        text={ANSWER}
        truncated={false}
        timestampMs={at(1)}
        onOpenTarget={onOpen}
      />
    </div>
  );
}

const meta: Meta = {
  title: "systems/session/components/session-messages/SessionMessages",
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Agent-to-agent messages in the transcript (`_uiux.md` S1–S3): the left-aligned session message card on the receiver, the “Sent to” card and the “Reply from” card on the sender. The operator bubble stays the only right-aligned surface.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const MessageCard: Story = { render: () => <MessageCardStates /> };

export const ReceiverTranscript: Story = { render: () => <ReceiverRhythm /> };

export const ReplyCard: Story = { render: () => <ReplyCardStates /> };

export const SentCard: Story = { render: () => <SentCardStates /> };

export const SenderTranscript: Story = { render: () => <SenderFlow /> };

/** The sender link opens the session; ⌘/Ctrl opens it in a new window. */
export const OpenSender: Story = {
  tags: ["play-fn"],
  render: () => (
    <div className="p-8">
      <SessionMessageCard
        sender={refactorBilling}
        delivery={null}
        replyRequested={false}
        timestampMs={at(0)}
        onOpenSender={onOpen}
      >
        <Text>{QUESTION}</Text>
      </SessionMessageCard>
    </div>
  ),
  play: async ({ canvasElement }) => {
    onOpen.mockClear();
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByRole("button", { name: "Refactor billing" }));
    await expect(onOpen).toHaveBeenCalledWith(refactorBilling, { newWindow: false });
  },
};
