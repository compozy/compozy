## 0.4.7 - 2026-10-31

### Bug Fixes

- Preserve clarify line breaks and make the clarification dock collapsible (#1062)
- Support User-owned session lifecycle actions in global scope (#1060)
- Verify Claude hosted MCP readiness before startup succeeds (#1066)
- Show only the title for open terminals, without a running mark (#1073)
- Flag child sessions that stop without delivering and make end-turn guidance conditional (#1069)
- Keep forked and continued sessions visible when hiding child sessions (#1077)
- Reuse stdio MCP server processes across discovery, status, and calls (#1081)
- Resolve MCP OAuth credentials from the declaring scope (#1080)
- Keep failed worktree creations visible and roll them back in order (#1082)
- Verify interrupted worktree checkouts under the lock and by branch (#1084)
- Report the Later count on the active facets read
- Keep the facets Later total optional in the API contract
- Label inherited MCP auth in listings and hold unreadable workspaces

### Features

- Archive a project from the top-bar project submenu (#1061)
- Zoom the Mermaid fullscreen viewer with Ctrl/Cmd+wheel, pinch, and keys at the cursor (#1059)
- Replace the folder browser with a directory picker input and centre onboarding step numbers (#1064)
- Open terminals in a project's managed worktree (#1067)
- Add a Waiting board column and let every Kanban column collapse (#1079)
- Park sessions in Later with an optional reason and return date (#1083)
