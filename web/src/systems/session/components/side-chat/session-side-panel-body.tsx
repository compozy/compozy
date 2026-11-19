import type { ReactNode } from "react";

import {
  Button,
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupTextarea,
  Spinner,
} from "@compozy/ui";
import { ArrowUp, RotateCcw } from "@compozy/ui/icons";

import { SIDE_CHAT_COPY } from "../../lib/session-side";

/** What the side chat panel body shows (S3 states). */
export type SessionSidePanelBodyState =
  /** An empty side chat: its first question creates it (US-001.AC-4, US-003.AC-1). */
  | { kind: "new"; draft: string; onDraftChange: (draft: string) => void; onSend: () => void }
  /** The question is on its way and the side chat is starting. */
  | { kind: "opening"; message: string }
  /** Starting failed or the first message was not admitted: Retry sends it once (US-001.EC-5). */
  | { kind: "failed"; message: string; error: string; onRetry: () => void; retrying?: boolean }
  /** The side chat or its list could not be read. */
  | { kind: "unavailable"; onRetry: () => void }
  /** The side chat's own interactive transcript and composer. */
  | { kind: "thread"; thread: ReactNode; stopped?: boolean };

/** The side chat panel's body. Pure: the drawer host decides the state and binds the thread. */
export function SessionSidePanelBody({ state }: { state: SessionSidePanelBodyState }) {
  switch (state.kind) {
    case "new":
      return (
        <div
          className="flex min-h-0 flex-1 flex-col justify-end p-3"
          data-testid="side-chat-new-panel"
        >
          <SessionSideQuestionField
            draft={state.draft}
            onDraftChange={state.onDraftChange}
            onSend={state.onSend}
          />
        </div>
      );
    case "opening":
      return (
        <div className="flex min-h-0 flex-1 flex-col gap-3 p-4" data-testid="side-chat-opening">
          <SessionSideQuestion message={state.message} />
          <p
            className="flex items-center gap-2 text-small-body text-muted-foreground"
            role="status"
          >
            <Spinner aria-hidden="true" className="size-3.5" />
            {SIDE_CHAT_COPY.opening}
          </p>
        </div>
      );
    case "failed":
      return (
        <div className="flex min-h-0 flex-1 flex-col gap-3 p-4" data-testid="side-chat-failed">
          <SessionSideQuestion message={state.message} />
          <div className="flex flex-col items-start gap-2" role="alert">
            <p className="text-small-body text-destructive">{state.error}</p>
            <Button
              data-testid="side-chat-retry"
              disabled={state.retrying}
              onClick={state.onRetry}
              size="sm"
              type="button"
              variant="outline"
            >
              <RotateCcw aria-hidden="true" data-icon="inline-start" />
              {SIDE_CHAT_COPY.retry}
            </Button>
          </div>
        </div>
      );
    case "unavailable":
      return (
        <div className="flex flex-col items-start gap-2 p-4" role="alert">
          <p className="text-small-body text-muted-foreground">{SIDE_CHAT_COPY.listFailed}</p>
          <Button onClick={state.onRetry} size="sm" type="button" variant="ghost">
            {SIDE_CHAT_COPY.retry}
          </Button>
        </div>
      );
    case "thread":
      return (
        <div className="flex min-h-0 flex-1 flex-col" data-testid="side-chat-thread">
          {state.stopped ? (
            <p className="px-4 pt-3 text-small-body text-muted-foreground" role="status">
              {SIDE_CHAT_COPY.stoppedNote}
            </p>
          ) : null}
          {state.thread}
        </div>
      );
  }
}

/** The question as the operator sent it, while the side chat has no transcript yet. */
function SessionSideQuestion({ message }: { message: string }) {
  return (
    <p
      className="self-end rounded-lg bg-surface-2 px-3 py-2 text-body whitespace-pre-wrap text-fg"
      data-testid="side-chat-question"
    >
      {message}
    </p>
  );
}

/** The first question of a new side chat: Enter sends, Shift+Enter breaks the line. */
function SessionSideQuestionField({
  draft,
  onDraftChange,
  onSend,
}: {
  draft: string;
  onDraftChange: (draft: string) => void;
  onSend: () => void;
}) {
  const empty = draft.trim().length === 0;
  return (
    <InputGroup variant="composer">
      <InputGroupTextarea
        aria-label={SIDE_CHAT_COPY.questionLabel}
        autoFocus
        data-testid="side-chat-question-input"
        onChange={event => onDraftChange(event.target.value)}
        onKeyDown={event => {
          if (event.key !== "Enter" || event.shiftKey || event.nativeEvent.isComposing) return;
          event.preventDefault();
          if (!empty) onSend();
        }}
        placeholder={SIDE_CHAT_COPY.questionPlaceholder}
        rows={2}
        value={draft}
      />
      <InputGroupAddon align="block-end" className="justify-end">
        <InputGroupButton
          aria-label={SIDE_CHAT_COPY.send}
          data-testid="side-chat-question-send"
          disabled={empty}
          onClick={onSend}
          size="send"
          type="button"
        >
          <ArrowUp strokeWidth={2} />
        </InputGroupButton>
      </InputGroupAddon>
    </InputGroup>
  );
}
