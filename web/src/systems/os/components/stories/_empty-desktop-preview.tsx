import { useEffect, useState } from "react";
import { fn } from "storybook/test";

import { storyAgentNames } from "@/storybook/fintech-scenario";
import { agentFixtures } from "@/systems/agent/mocks";
import { sessionStore } from "@/systems/session";

import { emptyDesktopDraftId } from "../../hooks/use-empty-desktop-prompt-runtime";
import { shortcutLabel } from "../../lib/window-manager-shortcuts";
import { OsEmptyDesktopView } from "../os-empty-desktop";

// Agents carry a category path, as seeded workspaces do: the composer trigger
// must still lead with the full agent name and leave the path to the list.
const PREVIEW_AGENTS = agentFixtures.map(agent => ({
  ...agent,
  category_path: ["Operations", "Launch week"],
}));

const submitPrompt = fn().mockName("onSubmit");
const pickProject = fn().mockName("onPickProject");

export interface EmptyDesktopPreviewProps {
  /** Global scope has no project to start a session in. */
  hasProject?: boolean;
  /** A session is starting from the sent prompt. */
  starting?: boolean;
  /** Text in the composer (the sent prompt while starting); empty by default. */
  prompt?: string;
}

/**
 * Story-only empty desktop: the production view and session composer on a
 * transcript-less runtime, with the send and project pick recorded as actions
 * instead of reaching a daemon.
 */
export function EmptyDesktopPreview({
  hasProject = true,
  starting = false,
  prompt = "",
}: EmptyDesktopPreviewProps) {
  const draftId = emptyDesktopDraftId(hasProject ? "ws_story" : null);
  const [agentName, setAgentName] = useState<string | null>(storyAgentNames.product);
  // Every story owns its draft slot's text, so one story's prompt never leaks into the next.
  useEffect(() => {
    if (prompt === "") sessionStore.trigger.composerDraftDiscarded({ sessionId: draftId });
    else sessionStore.trigger.composerDraftChanged({ sessionId: draftId, text: prompt });
  }, [draftId, prompt]);
  return (
    <OsEmptyDesktopView
      desktopName="Desktop 1"
      paletteShortcutLabel={shortcutLabel("meta+KeyK")}
      hasProject={hasProject}
      starting={starting}
      draftId={draftId}
      onSubmit={text => submitPrompt(text)}
      agents={PREVIEW_AGENTS}
      agentName={agentName}
      onAgentChange={setAgentName}
      onPickProject={pickProject}
    />
  );
}
