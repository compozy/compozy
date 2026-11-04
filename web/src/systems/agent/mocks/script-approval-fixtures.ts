/**
 * Script sources waiting for the operator's approval (#1258): the catalog's read of each script
 * and the places where the wait shows. Plain data, shared by prototypes and stories.
 */
import type { AgentHeartbeatSourceCatalog } from "../types";

type CatalogScript = AgentHeartbeatSourceCatalog["scripts"][number];

export const SCRIPT_SHA_CURRENT =
  "sha256:7f3a91c0d2b4e6f8a1c3e5b7d9f1a3c5e7b9d1f3a5c7e9b1d3f5a7c9e1b3d5e2";
export const SCRIPT_SHA_CHANGED =
  "sha256:9c2e14ab03f6d8e1a2b4c6d8e0f2a4b6c8d0e2f4a6b8c0d2e4f6a8b0c2d4e6f8";

/** A new script: executable, never approved, never read by a check. */
export const pendingScriptFixture: CatalogScript = {
  name: "merge-candidates",
  executable: true,
  sha256: SCRIPT_SHA_CURRENT,
  approved: false,
  approved_sha256: null,
  line_count: 42,
  last_modified: "2026-11-04T14:52:00Z",
};

/** The same script after an edit: the approved bytes no longer match the file. */
export const changedScriptFixture: CatalogScript = {
  ...pendingScriptFixture,
  name: "stalled-sessions",
  sha256: SCRIPT_SHA_CHANGED,
  approved_sha256: SCRIPT_SHA_CURRENT,
  line_count: 57,
};

export const approvedScriptFixture: CatalogScript = {
  ...pendingScriptFixture,
  approved: true,
  approved_sha256: SCRIPT_SHA_CURRENT,
};

/** One bell entry per script that waits, with the agent and project it belongs to. */
export interface ScriptApprovalNotice {
  id: string;
  script: CatalogScript;
  agentLabel: string;
  agentName: string;
  project: string;
  /** The instance is paused, so no check will surface the request on its own. */
  paused: boolean;
  /** When the script's current bytes appeared. */
  since: string;
  /** The approved bytes changed; the request is for the new content. */
  changed: boolean;
}

export const scriptApprovalNoticesFixture: ScriptApprovalNotice[] = [
  {
    id: "merge-train:merge-candidates",
    script: pendingScriptFixture,
    agentLabel: "Merge Train",
    agentName: "merge-train",
    project: "compozy",
    paused: false,
    since: "2026-11-04T14:52:00Z",
    changed: false,
  },
  {
    id: "session-supervisor:stalled-sessions",
    script: changedScriptFixture,
    agentLabel: "Session Supervisor",
    agentName: "session-supervisor",
    project: "compozy",
    paused: true,
    since: "2026-11-04T13:10:00Z",
    changed: true,
  },
];

/** The first lines of a script, as a review peek shows them. */
export const scriptPeekFixture = [
  "#!/usr/bin/env bash",
  "# Pull requests that are green and approved, oldest first.",
  "set -euo pipefail",
  "gh pr list --repo compozy/compozy --state open --json number,title,reviewDecision \\",
  "  --jq '.[] | select(.reviewDecision == \"APPROVED\")'",
].join("\n");
