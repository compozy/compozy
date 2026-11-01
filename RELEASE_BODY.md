## 0.4.8 - 2026-11-01

### Bug Fixes

- Keep the accent ink on the menubar Updating pill (#1089)
- Restore the usage tray before the bell and hold its slot while loading (#1090)
- Ellipsize long session titles in session message card headers (#1099)
- Remove the Skip to content link that appeared on window reactivation (#1096)
- Hide empty projects in project grouping behind a Show empty projects option (#1091)
- Authorize explicit cross-workspace session replies (#1103)
- Let operators resume and take over spawned sessions after the parent stops (#1098)
- Decode each Later reason character reference once and only when complete
- Keep shelf card edges inside the horizontal scroller (#1116)
- Move the message trail right and add sidebar tonal hierarchy (#1115)
- Clear the window when its open session is archived (#1111)
- Store only clean text in generated session titles (#1125)
- Make the spawn reaper's parent-stop reap atomic and conditional (#1123)
- Let sessions call OAuth-authenticated extension MCP servers (#1117)
- Address merge train review findings on stops, titles, and MCP lookup
- Close shell-wait guard gaps found in review
- Satisfy gocritic in the PowerShell option check
- Keep the explicit owner wording in the MCP ambiguity error
- Format the MCP ambiguity error for golines
- Keep exit-plan reads from emitting worktree.status_refreshed (#1121)

### Features

- Rich-text reason and date picker in the Move to Later dialog (#1093)
- Group session processes into trees (#1106)
- Hide child sessions on the Kanban board by default (#1124)
- Apply runtime selection changes to the live session immediately (#1108)
- Redirect foreground shell waits to CompozyOS monitors (#1127)
- Make session monitors durable across daemon restarts and trace their lifecycle (#1129)

### Refactoring

- Replace the in-repo prototype gallery with @compozy/proto (#1092)
- Align @compozy/ui with the bnui contract (#1126)

### Testing

- Cover captured-only AfterStop cleanup and lead-only Stop all for process trees
