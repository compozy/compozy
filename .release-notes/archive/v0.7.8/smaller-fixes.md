---
title: Current subagent usage, terminal cap recovery and more fixes
type: fix
---

- **Usage for resumed Claude subagents.** When Claude runs a finished background subagent again, its
  token and tool counts now show the current run instead of the highest figure from any earlier run.
  This applies to the subagent drawer, the CLI, HTTP and UDS, and the native subagent status tool.
  Existing subagents keep their recorded usage on upgrade.
- **Terminal cap dialog after a refused Close.** A Terminal window waiting behind the terminal limit
  dialog used to stay on "Opening a terminal" when the daemon refused the Close you chose there. The
  limit dialog now opens again, and dismissing it leaves the window on its empty state.
- **Empty peer lists explain themselves.** When an always-on agent's peer list is empty because
  every peer was hidden by authorization, `GET /api/agent-instances?peers=true`, the CLI and
  `compozy__agent_instance_list` now return a `peers_note` saying so, instead of looking like there
  are no peers. The note names no agents, IDs or projects.
- **Pull request monitors reject branches with spaces.** `compozy__monitor_create` with
  `kind=pull_request` now refuses a branch selector that contains whitespace anywhere, including
  leading or trailing spaces, instead of waiting for a pull request that can't exist.
- **Extension tools with line separator characters.** A TypeScript extension tool whose schema
  contained U+2028 or U+2029 in a property name or string value could fail to build or become
  unavailable. Those tools now work, and published schema digests and Loop pins are unchanged.
- **Local sockets for stdio MCP servers.** Stdio MCP servers run with their own isolated home folder,
  so a server that finds a local service's socket through your home folder can't see it. The MCP
  documentation now shows how to pass the absolute socket path through the server's own argument or
  environment variable.
