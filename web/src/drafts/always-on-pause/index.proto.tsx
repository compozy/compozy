import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { MainSession } from "../always-on-reports/session-views";
import {
  AwayPause,
  ContextFullBlocked,
  ContextFullRotated,
  PromptWhilePaused,
  ResumablePause,
  ResumedReceipt,
  WakeLost,
} from "./pause-views";

export const meta = definePrototype({
  title: "Always-on pause",
  spec: "stays-on-hardening",
  surface: "S2 S3",
  route: "/sessions/sess-81f2a04c1d7e9b30",
  states: [
    { id: "report", label: "S2 · Paused at the limit" },
    { id: "away", label: "S3 · Paused while you were away" },
    { id: "prompt", label: "S2 · You message it while paused" },
    { id: "resumed", label: "S2 · After you resume" },
    { id: "wake-lost", label: "A wake-up was lost" },
    { id: "context-blocked", label: "Context full, can't move yet" },
    { id: "context-rotated", label: "Context full, new conversation" },
  ],
});

export default function AlwaysOnPauseDraft({ state }: PrototypeProps) {
  switch (state) {
    case "away":
      return (
        <MainSession state="paused">
          <AwayPause />
        </MainSession>
      );
    case "prompt":
      return (
        <MainSession state="paused">
          <PromptWhilePaused />
        </MainSession>
      );
    case "resumed":
      return (
        <MainSession state="working">
          <ResumedReceipt />
        </MainSession>
      );
    case "wake-lost":
      return (
        <MainSession state="failed">
          <WakeLost />
        </MainSession>
      );
    case "context-blocked":
      return (
        <MainSession state="failed">
          <ContextFullBlocked />
        </MainSession>
      );
    case "context-rotated":
      return (
        <MainSession>
          <ContextFullRotated />
        </MainSession>
      );
    default:
      return (
        <MainSession state="paused">
          <ResumablePause />
        </MainSession>
      );
  }
}
