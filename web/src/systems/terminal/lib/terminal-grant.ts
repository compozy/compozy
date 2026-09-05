/**
 * Reading a remembered decision as a terminal permission.
 *
 * Terminal grants are not a separate store: remembered command inputs live
 * where every other tool decision lives, which is why this translates rather
 * than fetches.
 *
 * What the daemon records is a **digest** of the exact tool input — never the
 * input itself, and never a terminal id or a command line. So the row says what
 * the decision covers ("this exact input") and shows the digest as the evidence
 * of *which* one. Reading a name out of a hash is not possible, and pretending
 * otherwise would put a made-up terminal name next to a real revoke button.
 */

export type TerminalGrantKind = "command_shape";

export interface TerminalGrant {
  id: string;
  kind: TerminalGrantKind;
  /** Digest of the exact tool input this decision covers, `sha256:…`. */
  inputDigest: string;
  agentName: string;
  grantedAt: string;
}

/** The remembered-decision shape the daemon already stores terminal grants in. */
export interface ToolApprovalGrantLike {
  id: string;
  tool_id: string;
  decision: "allow" | "reject";
  agent_name?: string;
  input_digest?: string;
  created_at: string;
}

/** The daemon only ever stores this shape; anything else is not a digest. */
const DIGEST = /^sha256:[0-9a-f]{64}$/;

/**
 * Translates one remembered decision, or returns null to leave it generic.
 *
 * A rejection is not a grant, so it stays in the generic row where its own copy
 * already reads correctly — calling it "always allowed" would invert it.
 */
export function terminalGrantFromToolGrant(grant: ToolApprovalGrantLike): TerminalGrant | null {
  if (grant.decision !== "allow") return null;
  const kind = terminalGrantKind(grant.tool_id);
  if (!kind) return null;
  const digest = grant.input_digest && DIGEST.test(grant.input_digest) ? grant.input_digest : null;
  // Exec remembers are exact-input grants. A stored allow without a
  // digest is either a tool-wide mint (a second policy editor) or a decision
  // the runtime should never have produced — both stay on the generic row.
  if (!digest) return null;
  return {
    id: grant.id,
    kind,
    agentName: grant.agent_name ?? "any agent",
    grantedAt: grant.created_at,
    inputDigest: digest,
  };
}

/**
 * Broader-decision mint must not become a second terminal policy editor.
 * Exec allows are prompt-origin exact inputs only.
 */
export function isTerminalBroaderDecisionForbidden(toolId: string): boolean {
  return toolId === "compozy__terminal_exec";
}

function terminalGrantKind(toolId: string): TerminalGrantKind | null {
  if (toolId === "compozy__terminal_exec") return "command_shape";
  return null;
}
