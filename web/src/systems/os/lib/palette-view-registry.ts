/**
 * The command palette's built-in view renderers (ADR-003).
 *
 * A view is a scoped, list-shaped level the root palette can push. The generic
 * shell owns the input, the keyboard contract, selection, the empty state and
 * the footer, so a view supplies only data — that is what keeps the mechanism
 * generic instead of one view's shape hardened into the palette.
 *
 * View *membership* is the daemon's: the registry serves a `view` action per
 * available view and the palette offers exactly those. This module owns only
 * the render side, so an id the daemon serves without a renderer here resolves
 * to null and degrades honestly rather than crashing (BR-2).
 */
import type { ReactNode } from "react";

import { OS_APP_DESCRIPTORS, type OsAppDescriptor } from "./app-catalog";

/** Open id space: the daemon (and, from P6, extensions) name the views. */
export type PaletteViewId = string;

export interface PaletteViewDefinition {
  readonly id: PaletteViewId;
  /** Breadcrumb crumb and root entry label. */
  readonly title: string;
  readonly icon: OsAppDescriptor["icon"];
  /** Replaces the root placeholder while this view is the active level. */
  readonly placeholder: string;
  /** What ⏎ does here, rendered in the footer hint. */
  readonly enterHint: string;
  /** Dialog description for assistive technology. */
  readonly description: string;
}

export const PALETTE_VIEWS: Readonly<Record<string, PaletteViewDefinition>> = {
  sessions: {
    id: "sessions",
    title: "Sessions",
    icon: OS_APP_DESCRIPTORS.session.icon,
    placeholder: "Search sessions…",
    enterHint: "open session",
    description: "Filter sessions by state and open one",
  },
};

/** Null for a view id this client has no renderer for — the caller degrades. */
export function paletteViewDefinition(id: PaletteViewId): PaletteViewDefinition | null {
  return PALETTE_VIEWS[id] ?? null;
}

/** One activatable result. `value` is the row's stable identity, not a search string. */
export interface PaletteViewRow {
  readonly value: string;
  readonly testId: string;
  readonly node: ReactNode;
  readonly disabled?: boolean;
  onSelect(): void;
}

export interface PaletteViewContent {
  readonly rows: readonly PaletteViewRow[];
  /** Filter chrome rendered between the input and the results. */
  readonly header: ReactNode | null;
  /** Shown in place of the results when `rows` is empty — names why. */
  readonly empty: ReactNode;
  /**
   * Trailing truth about the list itself — what the view matched but did not
   * render, or which sources failed. Never a control, never selectable.
   */
  readonly note: ReactNode | null;
  /** What ⌫ does on an empty query: the view's own reset, else "back". */
  readonly backHint: string;
  /**
   * Changes whenever the view itself re-scoped the list (a filter, a breadth).
   * The shell keeps the keyboard selection across every other change, so this
   * is what separates "the operator narrowed the list" — land on the first
   * result — from "the catalog moved underneath them" — stay put.
   */
  readonly resetKey: string;
  /**
   * Runs before the stack pops. Returning `true` means the view consumed the
   * keystroke (it had a filter to clear); `false` pops one level.
   */
  onEmptyQueryBackspace(): boolean;
}

export interface PaletteViewControllerInput {
  readonly query: string;
  /** Closes the whole palette — call it when a row activates. */
  onDismiss(): void;
}
