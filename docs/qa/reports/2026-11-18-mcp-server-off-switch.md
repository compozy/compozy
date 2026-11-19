# MCP server off switch

An isolated daemon, the Web app, two counting Streamable HTTP MCP fixtures, and the
ACP fixture provider exercised the off switch for [#1850](https://github.com/compozy/compozy/issues/1850).
The lab used its own home, HTTP port, and Unix socket. The screenshots use only fixture data.

| Scenario | Result | Observation |
| --- | --- | --- |
| ET-web-mcp-status-matrix | Pass | Settings switched `echo` off and retained the definition and the neutral Off status after reload. The extension server stayed Ready. |
| ET-session-mcp-control-states | Pass | The open session retained the `echo` row with Off and its reason. Its companion retained two tools. The session check returned the same off row. |
| ET-cli-mcp-runtime-verbs | Pass | The CLI returned `off` after disable and check, then `on` and `ready` after enable and check. The owner-qualified extension toggle preserved its definition digest and runtime name. |
| RT-mcp-hot-activation-next-message | Pass | Three messages in the same open session saw two, one, then two usable servers. The off fixture's request count stayed at 30 through both checks and the next message. |

The owner-qualified extension check and HTTP definition read left its request count at 48 while
off. The workspace-profile `mcp.json` toggle left the user definition on and kept the project
profile's waiting approval through both transitions. The daemon suite independently verifies
that an approved server remains approved and receives zero requests when off.

The PR carries the inspected Settings and session screenshots and the CLI transcript capture.
The lab daemon, fixture process, and Web server were stopped after the walk.
