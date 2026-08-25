import { useState } from "react";

import { Button } from "@compozy/ui";

import { useCmdPaletteDeclarativeView } from "../hooks/use-cmd-palette-declarative-view";
import { useCmdPaletteProgramView } from "../hooks/use-cmd-palette-program-view";
import type { CmdPaletteDispatch } from "../hooks/use-cmd-palette-dispatch";
import { useOsPaletteDomainView } from "../hooks/use-os-palette-domain-view";
import { useOsPaletteSessionsView } from "../hooks/use-os-palette-sessions-view";
import { OS_APP_DESCRIPTORS } from "../lib/app-catalog";
import {
  parseExtensionName,
  paletteViewDefinition,
  type PaletteViewId,
} from "../lib/palette-view-registry";
import type { PaletteBreadcrumb } from "../lib/palette-view-stack";
import type { WindowManagerRegisteredClientView } from "../lib/window-manager-types";
import { useProfileLens, useProfilesPaletteView } from "@/systems/profiles";

import { OsPaletteViewShell } from "./os-palette-view-shell";
import type { OsPaletteDomainRow } from "../lib/os-palette-domain-search";

interface PaletteViewFrameProps {
  dispatch: CmdPaletteDispatch;
  breadcrumb: PaletteBreadcrumb;
  query: string;
  onQueryChange: (query: string) => void;
  onPop: () => void;
  onDismiss: () => void;
  client: WindowManagerRegisteredClientView | null;
}

/**
 * One registration = one frame: it names the view's controller and hands its
 * content to the shared shell. Everything visible and every keystroke belong to
 * the shell, so a second view is this file plus a controller — not a second
 * palette.
 */
function SessionsPaletteViewFrame({
  client: _client,
  dispatch: _dispatch,
  onDismiss,
  ...shell
}: PaletteViewFrameProps) {
  const content = useOsPaletteSessionsView({ query: shell.query, onDismiss });
  const definition = paletteViewDefinition("sessions");
  if (definition === null) return null;
  return <OsPaletteViewShell definition={definition} content={content} {...shell} />;
}

/**
 * Profiles renders identities, not catalog entries, so it names its own
 * controller. Switching happens inline; every mutation hands off to the shared
 * lifecycle dialogs, which is why nothing here touches a plan.
 */
function ProfilesPaletteViewFrame({
  client: _client,
  dispatch: _dispatch,
  onDismiss,
  ...shell
}: PaletteViewFrameProps) {
  const lens = useProfileLens();
  const content = useProfilesPaletteView({ query: shell.query, lens, onDismiss });
  const definition = paletteViewDefinition("profiles");
  if (definition === null) return null;
  return <OsPaletteViewShell definition={definition} content={content} {...shell} />;
}

function DomainPaletteViewFrame({
  client: _client,
  dispatch: _dispatch,
  onDismiss,
  openDomainRow,
  viewId,
  ...shell
}: PaletteViewFrameProps & { viewId: string; openDomainRow: (row: OsPaletteDomainRow) => void }) {
  const definition = paletteViewDefinition(viewId);
  const fallbackDefinition = definition ?? unavailableDefinition(viewId);
  const content = useOsPaletteDomainView(fallbackDefinition, {
    query: shell.query,
    onDismiss,
    openDomainRow,
  });
  return <OsPaletteViewShell definition={fallbackDefinition} content={content} {...shell} />;
}

function DeclarativePaletteViewFrame({
  client,
  dispatch,
  onDismiss,
  viewId,
  ...shell
}: PaletteViewFrameProps & { viewId: string }) {
  const program = useCmdPaletteProgramView({
    client,
    dispatch,
    onDismiss,
    onQueryChange: shell.onQueryChange,
    query: shell.query,
    viewId,
  });
  const declarative = useCmdPaletteDeclarativeView({
    dispatch,
    enabled: program.declarative,
    onDismiss,
    query: shell.query,
    viewId,
  });
  const model = program.declarative ? declarative : program;
  if (program.declarative && (model.timedOut || model.error)) {
    const source = parseExtensionName(viewId);
    return (
      <OsPaletteViewShell
        definition={model.definition}
        content={{
          kind: "list",
          rows: [],
          header: null,
          empty: (
            <div className="space-y-3 px-3 py-6 text-center text-small-body text-muted">
              <p>{source ? `${source} view unavailable.` : "View unavailable."}</p>
              <Button size="sm" variant="outline" onClick={model.retry}>
                Retry
              </Button>
            </div>
          ),
          note: null,
          backHint: "back",
          resetKey: `${viewId}:unavailable`,
          onEmptyQueryBackspace: () => false,
        }}
        loading={model.loading}
        {...shell}
      />
    );
  }
  return (
    <OsPaletteViewShell
      definition={model.definition}
      content={model.content}
      loading={model.loading}
      {...shell}
    />
  );
}

export interface OsPaletteViewStackProps {
  client: WindowManagerRegisteredClientView | null;
  dispatch: CmdPaletteDispatch;
  viewId: PaletteViewId;
  breadcrumb: PaletteBreadcrumb;
  onPop: () => void;
  onDismiss: () => void;
  openDomainRow: (row: OsPaletteDomainRow) => void;
}

/**
 * The palette while it is showing a pushed view.
 *
 * The query lives here rather than in the shell so that entering or leaving a
 * level starts the new level's search clean — the level owns the question being
 * asked, and a stale query would ask it of the wrong list.
 */
export function OsPaletteViewStack({
  client,
  dispatch,
  viewId,
  breadcrumb,
  onPop,
  onDismiss,
  openDomainRow,
}: OsPaletteViewStackProps) {
  const [ownedQuery, setOwnedQuery] = useState({ viewId, value: "" });
  if (ownedQuery.viewId !== viewId) {
    setOwnedQuery({ viewId, value: "" });
  }
  const definition = paletteViewDefinition(viewId);
  const shell = {
    breadcrumb,
    client,
    dispatch,
    query: ownedQuery.value,
    onQueryChange: (value: string) => setOwnedQuery({ viewId, value }),
    onPop,
    onDismiss,
  };
  if (viewId === "sessions") {
    return <SessionsPaletteViewFrame {...shell} />;
  }
  if (viewId === "profiles") {
    return <ProfilesPaletteViewFrame {...shell} />;
  }
  if (definition?.domainTitle) {
    return (
      <DomainPaletteViewFrame
        key={viewId}
        {...shell}
        openDomainRow={openDomainRow}
        viewId={viewId}
      />
    );
  }
  return <DeclarativePaletteViewFrame key={viewId} {...shell} viewId={viewId} />;
}

/** A view the catalog offers that this client cannot render — named, never blank. */
export function OsPaletteViewUnavailable({
  breadcrumb,
  onPop,
  viewId,
}: {
  breadcrumb: PaletteBreadcrumb;
  onPop: () => void;
  viewId: string;
}) {
  return (
    <OsPaletteViewShell
      breadcrumb={breadcrumb}
      content={{
        rows: [],
        header: null,
        empty: (
          <p className="px-3 py-6 text-center text-small-body text-muted">
            {parseExtensionName(viewId) ?? viewId} view is not available in this client.
          </p>
        ),
        note: null,
        backHint: "back",
        resetKey: viewId,
        onEmptyQueryBackspace: () => false,
      }}
      definition={unavailableDefinition(viewId)}
      query=""
      onPop={onPop}
      onQueryChange={() => {}}
    />
  );
}

function unavailableDefinition(viewId: string) {
  return {
    id: viewId,
    title: viewId,
    icon: OS_APP_DESCRIPTORS.session.icon,
    placeholder: "Search…",
    enterHint: "open",
    description: "This view is not available in this client",
  };
}
