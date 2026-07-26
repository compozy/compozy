import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { userEvent, within } from "storybook/test";

import { aghApiMock } from "@/storybook/openapi-msw";
import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";

/** Ref seeded by the default vault MSW fixtures — used to force a collision. */
const EXISTING_REF = "vault:providers/claude/default";

async function openCreateEditor(canvasElement: HTMLElement) {
  const canvas = within(canvasElement.ownerDocument.body);
  await userEvent.click(await canvas.findByTestId("vault-page-create"));
  return canvas;
}

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/vault/routes/Vault",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Full app-shell route stories for the vault secrets route, backed by OpenAPI-typed MSW handlers.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
  parameters: appRouteParameters("/vault"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Cards: Story = {
  args: {},
  parameters: appRouteParameters("/vault?view=cards"),
  render: () => <StorybookWorkspaceSetup />,
};

export const Empty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/vault"),
    ...storybookMswParameters({
      vault: [aghApiMock.get("/api/vault/secrets", () => HttpResponse.json({ secrets: [] }))],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/vault"),
    ...storybookMswParameters({
      vault: [
        aghApiMock.get("/api/vault/secrets", async () => {
          await delay("infinite");
          return HttpResponse.json({ secrets: [] });
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters("/vault"),
    ...storybookMswParameters({
      vault: [
        aghApiMock.get("/api/vault/secrets", () =>
          HttpResponse.json({ error: "vault unavailable" }, { status: 503 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/**
 * VC-08 capture target: the production create body — reference and value
 * sections with the write-only SecretField and the write-only boundary notice.
 */
export const CreateSecret: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/vault"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = await openCreateEditor(canvasElement);
    const ref = await canvas.findByTestId("settings-vault-editor-ref-input");
    await userEvent.clear(ref);
    await userEvent.type(ref, "vault:providers/openai/api-key");
    await userEvent.type(await canvas.findByTestId("settings-vault-editor-kind-input"), "api_key");
  },
};

/**
 * Duplicate ref, before confirmation: the overwrite Alert is shown and the
 * primary action stays disabled because PUT is an upsert.
 */
export const CreateSecretDuplicateRef: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/vault"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = await openCreateEditor(canvasElement);
    const ref = await canvas.findByTestId("settings-vault-editor-ref-input");
    await userEvent.clear(ref);
    await userEvent.type(ref, EXISTING_REF);
    await userEvent.type(await canvas.findByLabelText("Secret value"), "rotated-value");
  },
};

/** Duplicate ref, after confirmation: the write is unblocked. */
export const CreateSecretOverwriteConfirmed: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: appRouteParameters("/vault"),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = await openCreateEditor(canvasElement);
    const ref = await canvas.findByTestId("settings-vault-editor-ref-input");
    await userEvent.clear(ref);
    await userEvent.type(ref, EXISTING_REF);
    await userEvent.type(await canvas.findByLabelText("Secret value"), "rotated-value");
    await userEvent.click(await canvas.findByTestId("settings-vault-editor-overwrite-confirm"));
  },
};
