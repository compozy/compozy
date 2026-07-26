import { Check, Package } from "lucide-react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { WireCard, WireCardBody, WireCardFoot, WireCardHead } from "@agh/ui";

const meta: Meta<typeof WireCard> = {
  title: "components/custom/WireCard",
  component: WireCard,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Bordered protocol card, mirrors `.wire-card` (head/body/foot) in `docs/design/web-inspiration/styles/app.css`. Used to embed wire-protocol payloads (capabilities, receipts, and descriptors) inside message threads.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Capability: Story = {
  render: () => (
    <WireCard>
      <WireCardHead>
        <Package className="size-3" />
        <span>Capability</span>
        <span className="text-fg">rag.embed.bulk · v2</span>
      </WireCardHead>
      <WireCardBody>
        <div className="grid grid-cols-2 gap-3 text-eyebrow">
          <div>
            <span className="text-subtle">accepts:</span> <span className="text-fg">urls</span>
          </div>
          <div>
            <span className="text-subtle">emits:</span> <span className="text-fg">vector_ids</span>
          </div>
        </div>
      </WireCardBody>
      <WireCardFoot>
        <button
          className="eyebrow rounded-xs border border-line px-2 py-1 text-muted hover:text-fg"
          type="button"
        >
          Call capability
        </button>
        <button
          className="eyebrow rounded-xs border border-line px-2 py-1 text-muted hover:text-fg"
          type="button"
        >
          View schema
        </button>
      </WireCardFoot>
    </WireCard>
  ),
};

export const InlineReceipt: Story = {
  render: () => (
    <WireCard inline>
      <Check className="size-3 text-success" />
      <span className="font-mono text-eyebrow text-fg">receipt for direct#8471 · ok</span>
      <span className="ml-auto font-mono text-badge text-subtle">latency 212ms</span>
    </WireCard>
  ),
};
