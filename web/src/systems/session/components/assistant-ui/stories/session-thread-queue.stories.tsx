import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fireEvent, userEvent, waitFor, within } from "storybook/test";

import { SessionThread } from "@/components/assistant-ui/session-thread";
import { useSessionPageControls } from "@/hooks/routes/use-session-page-controls";
import { storybookMswParameters } from "@/storybook/msw";
import { SessionChatRuntimeProvider } from "@/systems/session/components/session-chat-runtime-provider";
import { SessionPromptRuntimeSelector } from "@/systems/session/components/session-prompt-runtime-selector";
import { SessionPromptRuntimeProvider } from "@/systems/session/contexts/session-prompt-runtime-context";
import { canPromptSession } from "@/systems/session/lib/session-running";
import { primarySessionFixture } from "@/systems/session/mocks";
import { sessionStore } from "@/systems/session/stores/session-store";

import {
  QUEUE_STORY_CAP,
  QUEUE_STORY_ENTRIES,
  QUEUE_STORY_FULL_ENTRIES,
  QUEUE_STORY_OTHER_ACTOR_ENTRIES,
  queueStoryClearedTranscript,
  queueStoryHandlers,
  type QueueStorySceneOptions,
  queueStorySession,
} from "./session-thread-story-queue";

const storyWorkspaceId = primarySessionFixture.workspace_id ?? "ws_alpha";

/**
 * The production wiring, as the session window composes it: the page-controls
 * hook owns the queue read model, the busy-send gate, the unconfirmed sends and
 * every mutation; the thread renders what it says. Nothing here is a stand-in.
 */
function QueueStoryHost() {
  const controls = useSessionPageControls(queueStorySession.id, queueStorySession, {
    workspaceId: storyWorkspaceId,
  });
  return (
    <SessionThread
      sessionId={queueStorySession.id}
      workspaceId={storyWorkspaceId}
      agentName={queueStorySession.agent_name}
      sessionState={queueStorySession.state}
      statusSession={queueStorySession}
      canPrompt={controls.canPrompt}
      onCancelPrompt={controls.handleCancelPrompt}
      onQueuePrompt={controls.handleQueuePrompt}
      onInterruptPrompt={controls.handleInterruptPrompt}
      onSteerPrompt={controls.handleSteerPrompt}
      isBusyInputPending={controls.isBusyInputPending}
      isSessionRunning={controls.isSessionRunning}
      stopPhase={controls.stopPhase}
      allowBusyInput={controls.allowBusyInput}
      busyInputDefaultMode={controls.busyInputDefaultMode}
      busyInputSteerDelivery={controls.busyInputSteerDelivery}
      queuedPrompts={controls.queuedPrompts}
      onRemoveQueuedPrompt={controls.handleRemoveQueuedPrompt}
      onReplaceQueuedPrompt={controls.handleReplaceQueuedPrompt}
      onSteerQueuedPrompt={controls.handleSteerQueuedPrompt}
      onClearQueue={controls.handleClearQueue}
      queueCap={controls.queueCap}
      unconfirmedSends={controls.unconfirmedSends}
      onRetryUnconfirmedSend={controls.handleRetryUnconfirmedSend}
      onDiscardUnconfirmedSend={controls.handleDiscardUnconfirmedSend}
      runtimeControl={<SessionPromptRuntimeSelector canPrompt={controls.canPrompt} />}
    />
  );
}

function scene(options: QueueStorySceneOptions) {
  return storybookMswParameters({ session: queueStoryHandlers(options) });
}

/**
 * Puts a draft into the Lexical composer as one paste (one editor update, no
 * per-keystroke races), retried until the draft reads back: the editor's
 * plugins register a beat after mount, and an edit that lands early is dropped.
 */
async function typeIntoComposer(canvas: ReturnType<typeof within>, text: string) {
  const input = await canvas.findByLabelText("Session prompt");
  await waitFor(async () => {
    await userEvent.click(input);
    await userEvent.clear(input);
    await userEvent.paste(text);
    await expect(input).toHaveTextContent(text);
  });
}

/**
 * Queue strip (S2, task_03 VC-01..06) inside the artboard's 860×560 session
 * window body: real `SessionQueueStrip` + composer over an MSW queue that
 * answers every verb. Each story is one contract state, reached either on load
 * or through the interaction its `play` performs.
 */
const meta: Meta<typeof QueueStoryHost> = {
  title: "systems/session/components/assistant-ui/SessionThread/Queue",
  component: QueueStoryHost,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "The strip fuses onto the composer top: a header with the count and the only Clear all, one 32px row per entry (mono position, owner attribution for another actor, one-line preview, Steer / edit / remove), a dispatching head row (“Sending…”, verbs absent), the client-local unconfirmed row with Retry, the full header at cap (Queue button and ⌘⏎ queue hint absent), and the inline clear confirmation. Every button runs the production handler against the story's MSW queue.",
      },
    },
  },
  // Drafts persist per session across stories in one browser; each scene
  // starts with the composer the way its contract shows it: empty.
  loaders: [
    () => {
      sessionStore.trigger.composerDraftDiscarded({ sessionId: queueStorySession.id });
      return {};
    },
  ],
  decorators: [
    // The session window's own provider stack (`session-window-view.tsx`): the
    // prompt-runtime provider for the composer footer, then the chat runtime.
    Story => (
      <SessionPromptRuntimeProvider
        canPrompt={canPromptSession(queueStorySession)}
        session={queueStorySession}
      >
        <SessionChatRuntimeProvider sessionId={queueStorySession.id} workspaceId={storyWorkspaceId}>
          <div
            className="flex h-[560px] w-[860px] max-w-full flex-col overflow-hidden rounded-window border border-line-focus bg-canvas shadow-window"
            data-testid="queue-story-window"
          >
            <Story />
          </div>
        </SessionChatRuntimeProvider>
      </SessionPromptRuntimeProvider>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-01 — three of the operator's own follow-ups, ordered as they will run; every verb live. */
export const Populated: Story = {
  parameters: scene({ entries: QUEUE_STORY_ENTRIES }),
};

/** VC-01 (§07) — Edit on a parked own row turns the row into the in-row editor; Save replaces atomically. */
export const EditingOwnRow: Story = {
  parameters: scene({ entries: QUEUE_STORY_ENTRIES }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const [edit] = await canvas.findAllByTestId("composer-queued-edit");
    await userEvent.click(edit!);
    await expect(await canvas.findByTestId("composer-queued-editor")).toBeVisible();
  },
};

/** VC-02 — agent-owned entries carry actor names and omit mutation controls. */
export const OtherActor: Story = {
  parameters: scene({ entries: QUEUE_STORY_OTHER_ACTOR_ENTRIES }),
};

/**
 * VC-03 — the head entry starts sending while it is being edited: the daemon
 * refuses the Save with `entry_dispatching`, the row reads “Sending…” with its
 * verbs gone, and the edited text lands in the composer as a fresh draft.
 */
export const DispatchingEditRefused: Story = {
  parameters: scene({ entries: QUEUE_STORY_ENTRIES, replace: "entry_dispatching" }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const [edit] = await canvas.findAllByTestId("composer-queued-edit");
    await userEvent.click(edit!);
    const editor = await canvas.findByTestId("composer-queued-editor");
    // One change event with the whole edited text: the controlled field takes
    // it in a single update, and a capture never sees a half-typed edit. A
    // change event does not depend on the document holding focus, which a
    // CDP-driven headless page without focus emulation never does — keyboard
    // and clipboard simulation would land on `body` there and the row would
    // save its original text.
    const edited = "Ship it with tests — and run the e2e lane too";
    fireEvent.change(editor, { target: { value: edited } });
    await waitFor(() => {
      if ((editor as HTMLTextAreaElement).value !== edited) {
        throw new Error("The queued-message editor did not take the edited text");
      }
    });
    await userEvent.click(canvas.getByTestId("composer-queued-edit-save"));
    await waitFor(async () => {
      await expect(canvas.getByTestId("composer-feedback-note")).toHaveTextContent(
        "already sending"
      );
    });
    await expect(await canvas.findByText("Sending…")).toBeVisible();
    await expect(await canvas.findByLabelText("Session prompt")).toHaveTextContent(
      "and run the e2e lane too"
    );
  },
};

/**
 * VC-04 — a queue send whose acknowledgment is lost: the browser keeps the
 * identity as the one client-local row (“Not confirmed”, no position) with
 * Retry — the single accent on the screen.
 */
export const Unconfirmed: Story = {
  parameters: scene({ entries: QUEUE_STORY_ENTRIES.slice(0, 1), prompt: "acknowledgment_lost" }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await typeIntoComposer(canvas, "Also run the migration equivalence suite");
    await userEvent.click(await canvas.findByTestId("composer-queue-button"));
    await expect(await canvas.findByTestId("composer-queued-retry")).toBeVisible();
  },
};

/**
 * VC-04 (resolved) — Retry replays the same message_id · idempotency_key; the
 * daemon had recorded it, so the row resolves and the composer note reads the
 * replayed outcome: nothing was sent twice.
 */
export const UnconfirmedRetryReplayed: Story = {
  parameters: scene({ entries: QUEUE_STORY_ENTRIES.slice(0, 1), prompt: "lost_then_replayed" }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await typeIntoComposer(canvas, "Also run the migration equivalence suite");
    await userEvent.click(await canvas.findByTestId("composer-queue-button"));
    await userEvent.click(await canvas.findByTestId("composer-queued-retry"));
    await waitFor(async () => {
      await expect(canvas.getByTestId("composer-feedback-suffix")).toHaveTextContent("replayed");
    });
  },
};

/** VC-05 — ten of ten: the header reads full, the strip scrolls inside its four-row cap, Queue and its hint are absent. */
export const Full: Story = {
  parameters: scene({ cap: QUEUE_STORY_CAP, entries: QUEUE_STORY_FULL_ENTRIES }),
};

/** A concurrent enqueue fills the last slot before this send; the refused draft stays intact. */
export const FullRefusedDraft: Story = {
  parameters: scene({
    cap: QUEUE_STORY_CAP,
    entries: QUEUE_STORY_FULL_ENTRIES.slice(0, -1),
    prompt: "queue_full",
  }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const draft = "Also verify the final migration before shipping";
    await typeIntoComposer(canvas, draft);
    await userEvent.click(await canvas.findByTestId("composer-queue-button"));
    await waitFor(() =>
      expect(canvas.getByTestId("composer-feedback-note")).toHaveTextContent("queue is full")
    );
    await expect(canvas.getByLabelText("Session prompt")).toHaveTextContent(draft);
    await expect(canvas.getByTestId("composer-steer-button")).toBeVisible();
  },
};

/** VC-06 — Clear all swaps the header for the inline question: destructive Clear all, quiet Keep, Esc keeps. */
export const ClearConfirm: Story = {
  parameters: scene({ entries: QUEUE_STORY_OTHER_ACTOR_ENTRIES }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("composer-queue-clear"));
    await expect(await canvas.findByTestId("composer-queue-clear-confirm")).toBeVisible();
  },
};

/** VC-06 (runs) — confirming clears the daemon queue; the strip is gone. */
export const Cleared: Story = {
  parameters: scene({ entries: QUEUE_STORY_ENTRIES }),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("composer-queue-clear"));
    await userEvent.click(await canvas.findByTestId("composer-queue-clear-confirm-button"));
    await waitFor(async () => {
      await expect(canvas.queryByTestId("composer-queued-prompts")).not.toBeInTheDocument();
    });
  },
};

/** VC-06 (trace) — after the clear, the conversation carries one marker per removed entry. */
export const ClearedTrace: Story = {
  parameters: scene({ entries: [], transcript: queueStoryClearedTranscript }),
};
