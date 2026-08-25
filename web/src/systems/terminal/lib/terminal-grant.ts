/**
 * Reading a remembered decision as a terminal permission.
 *
 * Terminal grants are not a separate store: typing rights and remembered
 * command inputs live where every other tool decision lives, which is why this
 * translates rather than fetches.
 *
 * What the daemon records is a **digest** of the exact tool input — never the
 * input itself, and never a terminal id or a command line. So the row says what
 * the decision covers ("this exact input") and shows the digest as the evidence
 * of *which* one. Reading a name out of a hash is not possible, and pretending
 * otherwise would put a made-up terminal name next to a real revoke button.
 */

/** Two genuinely different promises, both remembered in the same place. */
export type TerminalGrantKind = "typing" | "command_shape";

export interface TerminalGrant {
  id: string;
  kind: TerminalGrantKind;
  /**
   * The digest of the exact tool input this decision covers, `sha256:…`.
   *
   * Absent when the decision covers the whole tool rather than one input.
   */
  inputDigest?: string;
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
  // Typing is always scoped to one exact terminal generation — no autonomy
  // level and no admin path widens it. A stored typing decision without a
  // digest is therefore a decision the runtime should never have produced, and
  // this refuses to give it a friendly terminal-shaped reading: it falls back
  // to the generic remembered-decision row, where its tool id and its revoke
  // button are shown for exactly what they are.
  if (kind === "typing" && !digest) return null;
  return {
    id: grant.id,
    kind,
    agentName: grant.agent_name ?? "any agent",
    grantedAt: grant.created_at,
    ...(digest ? { inputDigest: digest } : {}),
  };
}

function terminalGrantKind(toolId: string): TerminalGrantKind | null {
  if (toolId === "compozy__terminal_write") return "typing";
  if (toolId === "compozy__terminal_exec" || toolId === "compozy__terminal_open") {
    return "command_shape";
  }
  return null;
}
