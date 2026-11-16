import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { ProviderLogo } from "@/systems/settings/components/provider-logo";

import { MCPAllowPanel, MCPCopyPanel } from "../mcp-redesign-shared/mcp-row-panels";
import { MCPSignInPanel } from "../mcp-redesign-shared/mcp-sign-in-panel";
import { AgentAttachDialog } from "./agent-attach-dialog";
import { AddServerDialog, ExtensionServerDialog, RemoveServerDialog } from "./server-dialogs";
import { SettingsBackdrop, type SettingsBackdropProps } from "./today-backdrop";

export const meta = definePrototype({
  title: "MCP redesign · Settings modals and panels",
  route: "/settings/mcp",
  states: [
    { id: "sign-in", label: "Sign in · Panel beside the row (replaces the modal)" },
    { id: "sign-in-connected", label: "Sign in · Connected" },
    { id: "review", label: "Project server · Review before allowing" },
    { id: "copy", label: "Coding agent server · Add to CompozyOS (replaces the dialog)" },
    { id: "add-empty", label: "Add server · Empty" },
    { id: "add-remote", label: "Add server · Remote URL" },
    { id: "add-local", label: "Add server · Command that needs a key" },
    { id: "add-json", label: "Add server · Pasted JSON from the docs" },
    { id: "edit", label: "Edit server" },
    { id: "remove", label: "Remove server" },
    { id: "remove-shadowed", label: "Remove server · Another copy takes over" },
    { id: "extension-settings", label: "Extension server · Set up" },
    { id: "agent-attach", label: "Agent · Pick MCP servers" },
    { id: "today", label: "Today · Settings page (unchanged)" },
  ],
});

const SIGN_IN_LINEAR = '[aria-label^="Sign in linear"]';

function propsFor(state: string): SettingsBackdropProps {
  switch (state) {
    case "sign-in":
      return {
        anchor: SIGN_IN_LINEAR,
        panel: <MCPSignInPanel server="linear" host="linear.app" phase="waiting" appliesOn="now" />,
      };
    case "sign-in-connected":
      return {
        anchor: SIGN_IN_LINEAR,
        panel: (
          <MCPSignInPanel
            server="linear"
            host="linear.app"
            phase="connected"
            appliesOn="now"
            toolCount={23}
            toolPreview={["Create issue", "Search issues", "Add comment"]}
          />
        ),
      };
    case "review":
      return {
        waiting: true,
        anchor: '[aria-label="Allow github-local"]',
        panel: (
          <MCPAllowPanel
            server="github-local"
            command="github-mcp-server stdio --read-only"
            appliesOn="now"
          />
        ),
      };
    case "copy":
      return {
        anchor: '[aria-label^="Copy context7 from Claude Code"]',
        panel: (
          <MCPCopyPanel
            server="context7"
            agent="Claude Code"
            mark={<ProviderLogo provider="claude" className="size-3.5" />}
            secrets={["CONTEXT7_API_KEY"]}
          />
        ),
      };
    case "add-empty":
      return { overlay: <AddServerDialog kind="empty" /> };
    case "add-remote":
      return { overlay: <AddServerDialog kind="remote" /> };
    case "add-local":
      return { overlay: <AddServerDialog kind="local" /> };
    case "add-json":
      return { overlay: <AddServerDialog kind="json" /> };
    case "edit":
      return { overlay: <AddServerDialog kind="edit" /> };
    case "remove":
      return { overlay: <RemoveServerDialog /> };
    case "remove-shadowed":
      return { overlay: <RemoveServerDialog shadowed /> };
    case "extension-settings":
      return { overlay: <ExtensionServerDialog /> };
    case "agent-attach":
      return { overlay: <AgentAttachDialog /> };
    default:
      return {};
  }
}

export default function MCPSettingsRedesignDraft({ state }: PrototypeProps) {
  return <SettingsBackdrop key={state} {...propsFor(state)} />;
}
