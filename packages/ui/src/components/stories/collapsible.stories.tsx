import type { Meta, StoryObj } from "@storybook/react-vite";
import { ChevronDownIcon, ChevronUpIcon } from "lucide-react";

import { Button } from "../button";
import { Card, CardContent, CardHeader, CardTitle } from "../card";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "../collapsible";

const meta: Meta<typeof Collapsible> = {
  title: "components/ui/Collapsible",
  component: Collapsible,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Minimal single-section disclosure. Prefer `Collapsible` over `Accordion` when there's only one expandable block.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const details = [
  "Spawned claude-code via ACP, exit code 0.",
  "Replayed 142 events from sessiondb.",
  "Dream consolidation skipped (too recent).",
];

export const Default: Story = {
  render: () => (
    <Card className="w-md">
      <CardHeader>
        <CardTitle>Session boot report</CardTitle>
      </CardHeader>
      <CardContent>
        <Collapsible>
          <CollapsibleTrigger
            render={
              <Button variant="ghost" size="sm" className="gap-1.5">
                <span className="group-data-panel-open/collapsible-trigger:hidden">
                  View timeline
                </span>
                <span className="hidden group-data-panel-open/collapsible-trigger:inline">
                  Hide timeline
                </span>
                <ChevronDownIcon className="size-4 group-data-panel-open/collapsible-trigger:hidden" />
                <ChevronUpIcon className="hidden size-4 group-data-panel-open/collapsible-trigger:inline" />
              </Button>
            }
            className="group/collapsible-trigger"
          />
          <CollapsibleContent>
            <ul className="mt-2 space-y-1 border-l border-border pl-3 text-sm text-muted-foreground">
              {details.map(line => (
                <li key={line}>{line}</li>
              ))}
            </ul>
          </CollapsibleContent>
        </Collapsible>
      </CardContent>
    </Card>
  ),
};

export const OpenByDefault: Story = {
  render: () => (
    <Card className="w-md">
      <CardHeader>
        <CardTitle>Session boot report</CardTitle>
      </CardHeader>
      <CardContent>
        <Collapsible defaultOpen>
          <CollapsibleTrigger
            className="group/collapsible-trigger"
            render={
              <Button variant="ghost" size="sm" className="gap-1.5">
                Hide timeline
                <ChevronDownIcon className="size-4 transition-transform group-data-panel-open/collapsible-trigger:rotate-180" />
              </Button>
            }
          />
          <CollapsibleContent>
            <ul className="mt-2 space-y-1 border-l border-border pl-3 text-sm text-muted-foreground">
              {details.map(line => (
                <li key={line}>{line}</li>
              ))}
            </ul>
          </CollapsibleContent>
        </Collapsible>
      </CardContent>
    </Card>
  ),
};
