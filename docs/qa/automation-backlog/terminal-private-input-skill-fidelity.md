# Terminal skill must route private input through the redacted handoff

- Source: BUG-20260901-private-passphrase-session-composer / `ET-terminal-agent-handoff-input`
- Why automate: the official skill can describe the redacted terminal tool correctly while an agent
  still chooses the visible session composer for a private value. Static prose checks cannot prove
  that the model follows the routing rule.
- Suggested layer: a fresh managed-agent scenario in `make test-e2e-runtime` that exposes the bundled
  skill and terminal toolset, asks for a private passphrase during a terminal workflow, and records the
  first input surface selected.
- Spec sketch: start a visible persistent terminal, prompt the agent to request one private passphrase,
  and require one pending redacted terminal input request before any ordinary clarification or session
  response asks for the value. Resolve it and assert only the length marker reaches retained output.
- Status: proposed
