import type { Meta, StoryObj } from "@storybook/react-vite";

import { PageContent } from "../page-content";

const meta: Meta<typeof PageContent> = {
  title: "components/custom/PageContent",
  component: PageContent,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Canonical main-pane content container — `max-w-content-max` + `px-9`. ListingPage and PageShell compose this so every route shares one horizontal gutter. Route identity lives in the OS window head (Topbar), not in the body.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="flex h-[360px] w-full flex-col overflow-y-auto border border-line bg-background">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Listing: Story = {
  render: () => (
    <PageContent density="listing">
      <p className="text-card-title font-semibold text-fg-strong">Page content</p>
      <p className="text-[13px] text-muted">Horizontal inset matches ListingPage and PageShell.</p>
    </PageContent>
  ),
};

export const Comfortable: Story = {
  render: () => (
    <PageContent density="comfortable">
      <p className="text-card-title font-semibold text-fg-strong">Comfortable</p>
      <p className="text-[13px] text-muted">
        Same px-9 gutter; denser gap/padding differs by density.
      </p>
    </PageContent>
  ),
};
