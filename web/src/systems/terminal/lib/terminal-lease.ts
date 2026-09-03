/**
 * What the lease means for the person looking at the screen.
 *
 * The daemon states the lease; this module only says how it reads. Nothing here
 * infers control from local facts like whether input happens to be enabled —
 * that would let the UI claim a state the daemon never granted.
 */

import type {
  TerminalActor,
  TerminalCapabilities,
  TerminalLeaseState,
  TerminalMode,
} from "../types";

const ANONYMOUS_OPERATOR_ID = "operator";

export type TerminalControlRead = "you" | "someone-else" | "owner-unknown" | "agent" | "nobody";

export interface TerminalLeaseView {
  read: TerminalControlRead;
  /** The chip's sentence, in the plain register. */
  label: string;
  /** The controller's own name, for the confirmation that displaces them. */
  controllerName: string | null;
  /** Whether this viewer's keystrokes may reach the program. */
  canType: boolean;
  /** Whether the one take-control affordance is offered at all. */
  canTakeControl: boolean;
  /** Displacing another person confirms by name; an agent never does. */
  requiresConfirmation: boolean;
  /** Whether releasing control back is offered. */
  canRelease: boolean;
}

export interface TerminalLeaseInput {
  lease: TerminalLeaseState;
  controller: TerminalActor | null;
  /** This browser's operator identity, as the daemon names it. */
  viewerId: string | null;
  mode: TerminalMode | null;
  capabilities: TerminalCapabilities;
}

/**
 * A pipe terminal and an execute-only platform both make control meaningless.
 * The affordances are absent in that case, never disabled: a greyed-out button
 * still claims the feature exists here.
 */
function controlIsPossible(input: TerminalLeaseInput): boolean {
  return input.mode !== "pipe" && input.capabilities.interactive;
}

export function terminalLeaseView(input: TerminalLeaseInput): TerminalLeaseView {
  const possible = controlIsPossible(input);
  const controller = input.controller;
  if (input.lease === "human_owned" && controller) {
    const isViewer = input.viewerId !== null && controller.id === input.viewerId;
    if (isViewer) {
      return {
        read: "you",
        label: "You're in control",
        controllerName: controller.id,
        canType: possible,
        canTakeControl: false,
        requiresConfirmation: false,
        canRelease: possible,
      };
    }
    // The chip names who holds control — "operator" only when the daemon has
    // no better name for the same local person than the anonymous identity.
    const name = controller.id === ANONYMOUS_OPERATOR_ID ? "The operator" : controller.id;
    return {
      read: "someone-else",
      label: `${name} is in control`,
      controllerName: controller.id === ANONYMOUS_OPERATOR_ID ? "the operator" : controller.id,
      canType: false,
      canTakeControl: possible,
      // Taking over another person asks first; taking over an agent never does.
      // `operator` is the same local person before a browser registration
      // refines that identity. Only another concrete browser/person confirms.
      requiresConfirmation: controller.id !== ANONYMOUS_OPERATOR_ID,
      canRelease: false,
    };
  }
  if (input.lease === "human_owned") {
    return {
      read: "owner-unknown",
      label: "Someone is in control",
      controllerName: null,
      canType: false,
      canTakeControl: possible,
      requiresConfirmation: true,
      canRelease: false,
    };
  }
  if (input.lease === "agent_owned") {
    const name = controller?.id ?? "The agent";
    return {
      read: "agent",
      // A pipe terminal has nothing to control — the honest sentence is
      // attribution, not a claim about a lease no one can contest.
      label: input.mode === "pipe" ? `${name} ran this` : `${name} is in control`,
      controllerName: controller?.id ?? null,
      canType: false,
      canTakeControl: possible,
      requiresConfirmation: false,
      canRelease: false,
    };
  }
  return {
    read: "nobody",
    label: "No one in control",
    controllerName: null,
    canType: false,
    canTakeControl: possible,
    requiresConfirmation: false,
    canRelease: false,
  };
}

/**
 * The attach mode this viewer should request.
 *
 * Watching is the default everywhere: a viewer only asks for the write lease
 * after an explicit take-control gesture, never because it happens to be free.
 */
export function terminalAttachModeFor(view: TerminalLeaseView): "read" | "write" {
  return view.read === "you" ? "write" : "read";
}
