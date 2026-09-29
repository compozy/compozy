/**
 * Plain labels for the tool and skill allow/deny lists, shared by the create
 * dialog and agent settings. Raw IDs live in the help tip as examples.
 */
export const AGENT_ACCESS_COPY = {
  title: "What this agent is allowed to do",
  tools: {
    label: "Allowed tools",
    description: "Leave empty to allow everything.",
    help: "Tool names or wildcards, e.g. compozy__skill_view or mcp__github__*.",
    placeholder: "Add a tool",
  },
  toolsets: {
    label: "Tool groups",
    description: "Groups of tools turned on for this agent.",
    help: "Tool group names, e.g. compozy__catalog.",
    placeholder: "Add a tool group",
  },
  denyTools: {
    label: "Blocked tools",
    description: "Stay off even when allowed above.",
    help: "Tool names or wildcards, e.g. compozy__task_*.",
    placeholder: "Add a tool",
  },
  disabledSkills: {
    label: "Turned-off skills",
    description: "Skills this agent won't use.",
    help: "Skill names, e.g. code-review or release-notes.",
    placeholder: "Add a skill",
  },
} as const;
