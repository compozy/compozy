export const WORKSPACE_SETUP_COPY = {
  dialog: {
    title: "Add workspace",
    description: "Choose the folder sessions run in.",
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
