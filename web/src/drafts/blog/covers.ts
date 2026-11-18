/** Post covers generated for the new brand; prompts live in packages/brand/prompts/blog. */
import agentFrameworkArchitecture from "./assets/agent-framework-architecture.jpg";
import aiAgentRetriesIdempotency from "./assets/ai-agent-retries-idempotency.jpg";
import autonomousCodingAgentSetup from "./assets/autonomous-coding-agent-setup.jpg";
import cursorVsClaudeCode from "./assets/cursor-vs-claude-code.jpg";
import definingAgentSessionsCompozyos from "./assets/defining-agent-sessions-compozyos.jpg";
import gitRepositoryBriefingPython from "./assets/git-repository-briefing-python.jpg";
import graphLoopEditorLocalGateway from "./assets/graph-loop-editor-local-gateway.jpg";
import introducingCompozyos from "./assets/introducing-compozyos.jpg";

const COVERS: Record<string, string> = {
  "agent-framework-architecture": agentFrameworkArchitecture,
  "ai-agent-retries-idempotency": aiAgentRetriesIdempotency,
  "autonomous-coding-agent-setup": autonomousCodingAgentSetup,
  "cursor-vs-claude-code": cursorVsClaudeCode,
  "defining-agent-sessions-compozyos": definingAgentSessionsCompozyos,
  "git-repository-briefing-python": gitRepositoryBriefingPython,
  "graph-loop-editor-local-gateway": graphLoopEditorLocalGateway,
  "introducing-compozyos": introducingCompozyos,
};

export function coverOf(slug: string) {
  return COVERS[slug] ?? introducingCompozyos;
}
