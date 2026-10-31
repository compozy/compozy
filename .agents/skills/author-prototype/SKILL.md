---
name: author-prototype
description: Author and verify prototypes built from the consumer component library.
---

# Author a prototype

Read the project's instructions and docs/design/prototyping.md first. Run `proto list --json`
and `proto flow list --json` to find existing views, states and flows. Select the app with
`--app <dir>` when necessary. Use `proto new --in src/components/view.tsx --spec <spec>` for a
component prototype, or `proto new <slug> --draft` for an exploration. Component prototypes
compose existing views; put intrinsic markup in the owning view. Drafts may contain markup.
Keep metadata literal, give every state an id and label, and use only configured specs/roles.
Never import API clients, authentication or query hooks in prototypes. Views receive props.

Run `proto check --json`, then render the state with `proto url <slug> --state <id> --bare`.
Capture evidence with `proto screenshot <slug> --state <id>` and `proto evidence <slug> --state <id>`.
Read review notes with `proto comments list --json` and `proto comments show <id> --json`.
Agents never approve or reopen reviews. Approval belongs to the human reviewer.
