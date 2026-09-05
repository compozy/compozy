/**
 * Busy-input surface — what a send during a running turn resolves to: the
 * daemon default mode, steer delivery, structured refusals, and the send
 * outcome envelope. Grouped here and re-exported from the session barrel.
 */
export {
  DEFAULT_SESSION_BUSY_INPUT_MODE,
  oppositeSessionBusyInputMode,
  sessionBusyInputDefaultMode,
  sessionSteerDelivery,
  type SessionBusyInputAction,
  type SessionBusyInputMode,
  type SessionSteerDelivery,
} from "./lib/session-busy-input";
export {
  describeSessionBusyInputRefusal,
  SessionBusyInputRefusalError,
  sessionBusyInputRefusalFromError,
  type SessionBusyInputRefusal,
  type SessionBusyInputRefusalCode,
} from "./lib/session-busy-input-refusal";
export {
  sessionSendOutcomeFromResult,
  type SessionSendDisposition,
  type SessionSendOutcome,
} from "./lib/session-send-outcome";
