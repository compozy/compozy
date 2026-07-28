export const WORKSPACE_SETUP_COPY = {
  onboarding: {
    eyebrow: "Workspace setup",
    title: "Start CompozyOS with a real workspace, not an empty shell.",
    description:
      "Register your global workspace to anchor CompozyOS immediately, or point it at a specific project root if this machine already has a working directory in play.",
    noteLabel: "First-run note",
    noteBody:
      "CompozyOS needs at least one registered workspace before sessions, knowledge, and workspace-local skills can behave predictably.",
  },
  dialog: {
    title: "Add workspace",
    description:
      "Choose the fastest way to bring a workspace into CompozyOS without leaving the shell.",
  },
  global: {
    title: "Use global workspace",
    badge: "HOME",
    description:
      "Register your OS home directory as the default CompozyOS workspace and start with one click.",
    action: "Use global workspace",
  },
  manual: {
    title: "Register workspace",
    badge: "PATH",
    description: "Add any project root by absolute path. CompozyOS will resolve and register it.",
    inputLabel: "Workspace path",
    inputPlaceholder: "/Users/name/project",
    action: "Register workspace",
    dividerLabel: "or",
  },
} as const;
