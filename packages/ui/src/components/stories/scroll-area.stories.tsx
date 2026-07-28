import type { Meta, StoryObj } from "@storybook/react-vite";

import { Card, CardContent, CardHeader, CardTitle } from "../card";
import { Separator } from "../separator";
import { ScrollArea, ScrollBar } from "../scroll-area";

const meta: Meta<typeof ScrollArea> = {
  title: "components/ui/ScrollArea",
  component: ScrollArea,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Custom scrollbar wrapper with vertical and horizontal modes. Renders a track + thumb only when the viewport actually overflows.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const events = Array.from({ length: 24 }, (_, i) => ({
  id: `evt-${i + 1}`,
  label: `ACP event ${i + 1}`,
  summary: "tool_use · bash, listing repository files under /internal",
}));

export const Default: Story = {
  render: () => (
    <Card className="w-88">
      <CardHeader>
        <CardTitle>Session feed</CardTitle>
      </CardHeader>
      <CardContent className="p-0">
        <ScrollArea className="h-64">
          <ul className="px-4 pb-4 text-sm">
            {events.map(event => (
              <li key={event.id} className="py-2">
                <p className="font-medium">{event.label}</p>
                <p className="text-xs text-muted-foreground">{event.summary}</p>
                <Separator className="mt-2" />
              </li>
            ))}
          </ul>
        </ScrollArea>
      </CardContent>
    </Card>
  ),
};

export const Overflow: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "Viewport overflow surfaces the custom rounded thumb on a translucent track without shifting layout.",
      },
    },
  },
  render: () => (
    <div className="h-56 w-64 rounded-lg border bg-card">
      <ScrollArea className="h-full">
        <ol className="px-4 py-3 text-sm">
          {Array.from({ length: 40 }, (_, i) => (
            <li key={i} className="py-1">
              Row {String(i + 1).padStart(2, "0")}
            </li>
          ))}
        </ol>
      </ScrollArea>
    </div>
  ),
};

export const Horizontal: Story = {
  render: () => (
    <ScrollArea className="w-md rounded-lg border bg-card">
      <div className="flex gap-3 p-3">
        {events.slice(0, 12).map(event => (
          <Card key={event.id} className="w-48 shrink-0">
            <CardHeader className="p-3">
              <CardTitle className="text-sm">{event.label}</CardTitle>
            </CardHeader>
            <CardContent className="p-3 pt-0 text-xs text-muted-foreground">
              {event.summary}
            </CardContent>
          </Card>
        ))}
      </div>
      <ScrollBar orientation="horizontal" />
    </ScrollArea>
  ),
};
