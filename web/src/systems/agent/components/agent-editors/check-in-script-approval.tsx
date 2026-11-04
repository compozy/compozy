/**
 * The notice above Sources for each declared script that waits for the operator. It reads the
 * catalog, never the poll state: a new or paused source is never checked, so it would never ask.
 */
import { FileCode, SquareTerminal } from "@compozy/ui/icons";

import {
  Alert,
  AlertActions,
  AlertDescription,
  AlertMeta,
  AlertTitle,
  Button,
  Time,
} from "@compozy/ui";

import type { CheckInsEditorModel } from "../../hooks/use-check-ins-editor";
import { checkInScriptsAwaitingApproval } from "../../lib/agent-check-in-sources";
import type { AgentHeartbeatSourceCatalog } from "../../types";

type CatalogScript = AgentHeartbeatSourceCatalog["scripts"][number];

function waitingCopy(script: CatalogScript, paused: boolean, agentLabel: string) {
  if (script.approved_sha256) {
    return "The file changed since you approved it, so Heartbeat stopped running it. Approve the new version to bring it back.";
  }
  if (paused) {
    return `Heartbeat runs it on this computer at each check-in. Approve it now and it runs as soon as you resume ${agentLabel}.`;
  }
  return "Heartbeat runs it on this computer at each check-in. It stays off until you approve this version.";
}

function ScriptApprovalNotice({
  script,
  model,
  agentLabel,
}: {
  script: CatalogScript;
  model: CheckInsEditorModel;
  agentLabel: string;
}) {
  const { approve, approving, openScript } = model.sources;
  const busy = approving === script.name;
  const path = script.path;
  return (
    <Alert
      aria-label={`${script.name} needs your approval`}
      data-testid="check-in-script-approval"
      role="region"
    >
      <SquareTerminal aria-hidden="true" />
      <AlertTitle>
        {script.approved_sha256
          ? `Approve the new version of ${script.name}`
          : `Approve ${script.name} to let it run`}
      </AlertTitle>
      <AlertDescription>{waitingCopy(script, model.paused, agentLabel)}</AlertDescription>
      <AlertMeta>
        {script.line_count ? (
          <span>
            {script.line_count} {script.line_count === 1 ? "line" : "lines"}
          </span>
        ) : null}
        {script.last_modified ? (
          <span>
            Changed <Time iso={script.last_modified} />
          </span>
        ) : null}
      </AlertMeta>
      <AlertActions>
        {openScript && path ? (
          <Button onClick={() => openScript(path)} size="sm" type="button" variant="ghost">
            <FileCode aria-hidden="true" data-icon="inline-start" />
            Open script
          </Button>
        ) : null}
        <Button
          aria-busy={busy || undefined}
          disabled={busy}
          onClick={() => approve(script.name, script.sha256)}
          size="sm"
          type="button"
        >
          Approve
        </Button>
      </AlertActions>
    </Alert>
  );
}

/** One notice per declared script whose current bytes wait, in the file's order. */
export function CheckInScriptApprovals({
  model,
  agentLabel,
}: {
  model: CheckInsEditorModel;
  agentLabel: string;
}) {
  const { catalog, settings, readOnly } = model;
  if (readOnly || !catalog) return null;
  const waiting = checkInScriptsAwaitingApproval(settings.sources, catalog).flatMap(name => {
    const script = catalog.scripts.find(item => item.name === name);
    return script ? [script] : [];
  });
  if (waiting.length === 0) return null;
  return (
    <div className="flex flex-col gap-3">
      {waiting.map(script => (
        <ScriptApprovalNotice
          agentLabel={agentLabel}
          key={script.name}
          model={model}
          script={script}
        />
      ))}
    </div>
  );
}
