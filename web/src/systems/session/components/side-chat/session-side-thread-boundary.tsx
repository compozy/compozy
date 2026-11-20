import type { ReactNode } from "react";

import { SessionClearContext } from "../../contexts/session-clear-context-value";
import { SessionDeriveContext } from "../../contexts/session-derive-context-value";
import { SessionForkContext } from "../../contexts/session-fork-context-value";
import { SessionOriginContext } from "../../contexts/session-origin-context-value";
import { SessionReportSendContext } from "../../lib/session-report-send";

/**
 * The side chat transcript renders inside its session window, whose hosts belong to the main
 * session. This boundary takes them away: a side chat cannot be forked or continued, so "Fork from
 * here" and Continue are absent, not disabled (S6, US-011.AC-2); the window's clear dialog, origin
 * pill and report sender stay with the main session.
 */
export function SessionSideThreadBoundary({ children }: { children: ReactNode }) {
  return (
    <SessionForkContext value={null}>
      <SessionDeriveContext value={null}>
        <SessionClearContext value={null}>
          <SessionOriginContext value={null}>
            <SessionReportSendContext value={null}>{children}</SessionReportSendContext>
          </SessionOriginContext>
        </SessionClearContext>
      </SessionDeriveContext>
    </SessionForkContext>
  );
}
