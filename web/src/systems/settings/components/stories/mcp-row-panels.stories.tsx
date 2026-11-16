import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";

import { Popover, PopoverPanelContent } from "@compozy/ui";

import { claudeRuntimeMCPFixture } from "../../mocks/runtime-mcp-fixtures";
import { MCPAllowPanel } from "../mcp-panels/mcp-allow-panel";
import { MCPCopyPanel } from "../mcp-panels/mcp-copy-panel";
import { MCPIssuePanel } from "../mcp-panels/mcp-issue-panel";
import { MCPSignInPanel, type MCPSignInPhase } from "../mcp-panels/mcp-sign-in-panel";
import { ProviderLogo } from "../provider-logo";

const meta: Meta = {
  title: "systems/settings/components/MCPRowPanels",
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "The panels that open beside an MCP server's row button: sign in (every phase of the store), review a project server before allowing it, see why a server isn't working, and add a coding agent's server to CompozyOS.",
      },
    },
  },
};

export default meta;
type Story = StoryObj;

const noop = () => undefined;

function Open({ children }: { children: ReactNode }) {
  return (
    <Popover open modal={false}>
      <PopoverPanelContent>{children}</PopoverPanelContent>
    </Popover>
  );
}

function signIn(phase: MCPSignInPhase, extra: Partial<Parameters<typeof MCPSignInPanel>[0]> = {}) {
  return (
    <Open>
      <MCPSignInPanel
        server="linear"
        host="mcp.linear.app"
        phase={phase}
        url="https://mcp.linear.app/authorize?state=x"
        onClose={noop}
        onContinue={noop}
        onRetry={noop}
        onOpenAgain={noop}
        onRequestPaste={noop}
        onSubmitPaste={noop}
        {...extra}
      />
    </Open>
  );
}

export const SignInStarting: Story = { render: () => signIn("starting", { url: undefined }) };
export const SignInWaiting: Story = { render: () => signIn("waiting") };
export const SignInPaste: Story = { render: () => signIn("paste", { manual: true }) };
export const SignInConnected: Story = { render: () => signIn("connected", { toolCount: 23 }) };
export const SignInFailed: Story = {
  render: () => signIn("failed", { error: "The provider refused the sign-in." }),
};
export const SignInMoreAccess: Story = {
  render: () => signIn("scopes", { scopes: ["issues:read", "issues:write"] }),
};

export const Allow: Story = {
  render: () => (
    <Open>
      <MCPAllowPanel
        server="github-local"
        reason="added"
        launch={{ kind: "command", value: "github-mcp-server stdio --read-only" }}
        onAllow={noop}
        onClose={noop}
      />
    </Open>
  ),
};

export const NotWorking: Story = {
  render: () => (
    <Open>
      <MCPIssuePanel
        server="postgres"
        summary="It started but offered no tools"
        reason="connection refused: 127.0.0.1:5432"
        onCheck={noop}
        onOpenSettings={noop}
        onClose={noop}
      />
    </Open>
  ),
};

const copyServer =
  claudeRuntimeMCPFixture.servers.find(
    server => server.copyable && server.env_keys.length + server.header_keys.length > 0
  ) ?? claudeRuntimeMCPFixture.servers[0]!;

export const AddToCompozy: Story = {
  render: () => (
    <Open>
      <MCPCopyPanel
        server={copyServer}
        runtimeName={claudeRuntimeMCPFixture.display_name}
        mark={<ProviderLogo provider="claude" className="size-3.5" />}
        canTurnOff
        profile={null}
        workspaceId="ws-platform"
        submitting={false}
        failure={null}
        onSubmit={noop}
        onClose={noop}
      />
    </Open>
  ),
};
