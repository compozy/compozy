Goal (incl. success criteria):

- Extract the target information architecture and page/component model from `docs/design/daemon-mockup` only, for the official daemon web UI.
- Success means a concise report with: page/screen inventory, reusable layout/component patterns, key data entities/user flows implied by the mockup, and file references.

Constraints/Assumptions:

- Read-only scope for `docs/design/daemon-mockup`; do not modify mockup files.
- Use only artifacts inside that directory for the report.
- Keep report concise and grounded in file evidence.

Key decisions:

- Treat `Compozy Daemon Workspace.html` + `src/*.jsx` + `colors_and_type.css` as the source of truth.
- The mockup represents a single-shell app with route-driven screens and shared primitives, not a set of unrelated pages.

State:

- Context gathered from the mockup bundle and design tokens.

Done:

- Located all files under `docs/design/daemon-mockup`.
- Read the shell, page modules, shared data, icon/primitives, design tokens, and HTML entrypoint.

Now:

- Produce the requested concise report.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None blocking.

Working set (files/ids/commands):

- `docs/design/daemon-mockup/Compozy Daemon Workspace.html`
- `docs/design/daemon-mockup/colors_and_type.css`
- `docs/design/daemon-mockup/src/{dashboard.jsx,data.jsx,icons.jsx,memory.jsx,reviews.jsx,runs.jsx,shell.jsx,spec.jsx,task_detail.jsx,tasks.jsx,workflows.jsx}`
