import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import type { UseMCPAuthorizeReturn } from "../../hooks/use-mcp-authorize";
import { MCPAuthorizeDialog } from "../mcp-authorize-dialog";

const scopeReviewAuthorize: UseMCPAuthorizeReturn = {
  phase: "reviewing_scopes",
  server: "Linear",
  begin: null,
  error: null,
  prior: { status: "authenticated", tokenPresent: true },
  mode: "automatic",
  approvedScopes: ["issues:write", "comments:write", "projects:read"],
  requestAuthorize: fn(),
  confirmScopeEscalation: fn(),
  retryBegin: fn(),
  enterManual: fn(),
  submitManual: fn(),
  acknowledgeStatus: fn(),
  cancel: fn(),
};

const meta: Meta<typeof MCPAuthorizeDialog> = {
  title: "systems/settings/components/MCPAuthorizeDialog",
  component: MCPAuthorizeDialog,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Reviews additional OAuth scopes before the daemon starts a provider authorization.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Requires an explicit operator decision before requesting additional provider access.
 */
export const ScopeEscalationReview: Story = {
  args: {
    authorize: scopeReviewAuthorize,
    scope: "workspace",
    server: null,
  },
};
