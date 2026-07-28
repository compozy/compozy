import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import { SearchContent } from "@/systems/session/components/tool-renderers/search-content";
import { emptySearchToolMessageFixture, searchToolMessageFixture } from "@/systems/session/mocks";

const meta: Meta<typeof SearchContent> = {
  title: "systems/session/components/tool-renderers/SearchContent",
  component: SearchContent,
  parameters: {
    layout: "centered",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function SearchFrame({ children }: { children: React.ReactNode }) {
  return (
    <CenteredSurface>
      <div className="w-full max-w-3xl rounded-2xl border border-line bg-canvas p-4">
        {children}
      </div>
    </CenteredSurface>
  );
}

export const Default: Story = {
  args: {},
  render: () => (
    <SearchFrame>
      <SearchContent message={searchToolMessageFixture} />
    </SearchFrame>
  ),
};

export const EmptyResultSet: Story = {
  args: {},
  render: () => (
    <SearchFrame>
      <SearchContent message={emptySearchToolMessageFixture} />
    </SearchFrame>
  ),
};
