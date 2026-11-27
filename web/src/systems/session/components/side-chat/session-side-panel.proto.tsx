import { Eyebrow, PageContent } from "@compozy/ui";

import { definePrototype, type PrototypeProps } from "@protohq/cli";

import { SessionThreadSkeleton } from "@/components/assistant-ui/session-thread-skeleton";

import type { SessionSideItem } from "../../adapters/session-side-api";
import { sideMainStatus, type SideMainStatus } from "../../lib/session-side";
import {
  sideChatAnsweringFixture,
  sideChatListFixture,
  sideChatLongListFixture,
  sideChatMainSession,
  sideChatMainSessionFixture,
  sideChatPermissionFixture,
  sideChatQuestionFixture,
  sideChatStoppedFixture,
} from "../../mocks/side-chat-fixtures";
import { SessionSideCloseDialog } from "./session-side-close-dialog";
import { SessionSideHeader } from "./session-side-header";
import { SessionSidePanelBody, type SessionSidePanelBodyState } from "./session-side-panel-body";
import { SessionSideToggle } from "./session-side-toggle";

export const meta = definePrototype({
  title: "Side chat panel",
  spec: "side-chat",
  surface: "S3",
  route: "/sessions/$id",
  states: [
    { id: "new", label: "New side chat, focused composer" },
    { id: "opening", label: "Starting" },
    { id: "answering", label: "Answering, main working" },
    { id: "idle", label: "Idle, main finished" },
    { id: "failed", label: "Start failed with Retry" },
    { id: "permission", label: "Side permission pending" },
    { id: "main-status", label: "Main status values" },
    { id: "close-confirm", label: "Close while answering" },
    { id: "stopped", label: "Stopped, resumes on send" },
    { id: "long-switcher", label: "Long switcher" },
    { id: "toggle", label: "Head toggle states" },
  ],
});

const ignore = () => undefined;

interface Scenario {
  sides: readonly SessionSideItem[];
  currentId: string | null;
  pendingTitle?: string;
  status: SideMainStatus | null;
  body: SessionSidePanelBodyState;
  closeConfirm?: boolean;
}

const thread = <SessionThreadSkeleton />;
const working = sideMainStatus(sideChatMainSessionFixture, true);

const scenarios: Record<string, Scenario> = {
  new: {
    sides: sideChatListFixture,
    currentId: null,
    status: working,
    body: { kind: "new", draft: "", onDraftChange: ignore, onSend: ignore },
  },
  opening: {
    sides: sideChatListFixture,
    currentId: null,
    pendingTitle: sideChatQuestionFixture,
    status: working,
    body: { kind: "opening", message: sideChatQuestionFixture },
  },
  answering: {
    sides: sideChatListFixture,
    currentId: sideChatAnsweringFixture.id,
    status: working,
    body: { kind: "thread", thread },
  },
  idle: {
    sides: sideChatListFixture,
    currentId: sideChatListFixture[1]!.id,
    status: sideMainStatus(sideChatMainSession("done"), true),
    body: { kind: "thread", thread },
  },
  failed: {
    sides: sideChatListFixture,
    currentId: null,
    pendingTitle: sideChatQuestionFixture,
    status: working,
    body: {
      kind: "failed",
      message: sideChatQuestionFixture,
      error: "Couldn't open the side chat.",
      onRetry: ignore,
    },
  },
  permission: {
    sides: [sideChatPermissionFixture, ...sideChatListFixture],
    currentId: sideChatPermissionFixture.id,
    status: working,
    body: { kind: "thread", thread },
  },
  "close-confirm": {
    sides: sideChatListFixture,
    currentId: sideChatAnsweringFixture.id,
    status: working,
    body: { kind: "thread", thread },
    closeConfirm: true,
  },
  stopped: {
    sides: sideChatListFixture,
    currentId: sideChatStoppedFixture.id,
    status: sideMainStatus(sideChatMainSession("stopped", "stopped"), true),
    body: { kind: "thread", thread, stopped: true },
  },
  "long-switcher": {
    sides: sideChatLongListFixture,
    currentId: sideChatLongListFixture[0]!.id,
    status: working,
    body: { kind: "thread", thread },
  },
};

const MAIN_STATUS_BADGES = [
  ["running", "active"],
  ["waiting-for-auth", "active"],
  ["waiting-for-input", "active"],
  ["done", "active"],
  ["failed", "active"],
  ["stopped", "stopped"],
] as const;

function SidePanel({ scenario }: { scenario: Scenario }) {
  return (
    <PageContent
      className="flex h-[640px] max-w-md flex-col border-l border-line"
      density="compact"
    >
      <SessionSideHeader
        currentId={scenario.currentId}
        mainStatus={scenario.status}
        onClose={scenario.currentId ? ignore : undefined}
        onHide={ignore}
        onNew={ignore}
        onSelect={ignore}
        pendingTitle={scenario.pendingTitle}
        sides={scenario.sides}
      />
      <SessionSidePanelBody state={scenario.body} />
      <SessionSideCloseDialog
        closing={false}
        onConfirm={ignore}
        onOpenChange={ignore}
        open={scenario.closeConfirm === true}
      />
    </PageContent>
  );
}

export default function SessionSidePanelPrototype({ state }: PrototypeProps) {
  if (state === "main-status") {
    return (
      <PageContent className="flex max-w-md flex-col gap-2" density="compact">
        {MAIN_STATUS_BADGES.map(([badge, sessionState]) => (
          <SessionSideHeader
            currentId={sideChatAnsweringFixture.id}
            key={badge}
            mainStatus={sideMainStatus(sideChatMainSession(badge, sessionState), true)}
            onClose={ignore}
            onHide={ignore}
            onNew={ignore}
            onSelect={ignore}
            sides={sideChatListFixture}
          />
        ))}
        <Eyebrow>Unknown: reconnecting, no chip</Eyebrow>
        <SessionSideHeader
          currentId={sideChatAnsweringFixture.id}
          mainStatus={sideMainStatus(sideChatMainSessionFixture, false)}
          onClose={ignore}
          onHide={ignore}
          onNew={ignore}
          onSelect={ignore}
          sides={sideChatListFixture}
        />
      </PageContent>
    );
  }
  if (state === "toggle") {
    return (
      <PageContent className="flex max-w-sm flex-col items-start gap-3" density="compact">
        <Eyebrow>No side chats</Eyebrow>
        <SessionSideToggle attention={false} count={0} onToggle={ignore} open={false} />
        <Eyebrow>Three side chats</Eyebrow>
        <SessionSideToggle attention={false} count={3} onToggle={ignore} open={false} />
        <Eyebrow>A side chat needs you</Eyebrow>
        <SessionSideToggle attention count={4} onToggle={ignore} open={false} />
        <Eyebrow>Panel open</Eyebrow>
        <SessionSideToggle attention={false} count={3} onToggle={ignore} open />
      </PageContent>
    );
  }
  return <SidePanel scenario={scenarios[state] ?? scenarios.answering!} />;
}
