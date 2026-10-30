// Processes surface: the composer control, "Processes left running", and the
// stop/archive choice about the processes a session left running.
export { SessionComposerProcesses } from "./components/session-composer-processes";
export { ProcessesLeftRunningHost } from "./components/processes-left-running-host";
export {
  SessionProcessChoiceDialog,
  type SessionProcessChoiceDialogProps,
} from "./components/session-process-choice-dialog";
export type {
  SessionProcessGroupPayload,
  SessionProcessPayload,
  SessionProcessPolicy,
} from "./adapters/session-processes-api";
