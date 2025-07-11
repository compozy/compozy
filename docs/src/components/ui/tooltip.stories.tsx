import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { HelpCircle, Info, Settings } from "lucide-react";
import { Button } from "./button";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "./tooltip";

const meta = {
  title: "UI/Tooltip",
  component: Tooltip,
  parameters: {
    layout: "centered",
  },
  tags: ["autodocs"],
  decorators: [
    Story => (
      <TooltipProvider>
        <Story />
      </TooltipProvider>
    ),
  ],
} satisfies Meta<typeof Tooltip>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button variant="outline">Hover me</Button>
      </TooltipTrigger>
      <TooltipContent>
        <p>This is a tooltip</p>
      </TooltipContent>
    </Tooltip>
  ),
};

export const WithIcon: Story = {
  render: () => (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button variant="outline" size="icon">
          <HelpCircle className="h-4 w-4" />
        </Button>
      </TooltipTrigger>
      <TooltipContent>
        <p>Get help and support</p>
      </TooltipContent>
    </Tooltip>
  ),
};

export const LongContent: Story = {
  render: () => (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button variant="outline">
          <Info className="h-4 w-4 mr-2" />
          Complex Feature
        </Button>
      </TooltipTrigger>
      <TooltipContent className="max-w-xs">
        <p>
          This is a more detailed tooltip that explains a complex feature. It can contain multiple
          lines of text and provides comprehensive information about the functionality.
        </p>
      </TooltipContent>
    </Tooltip>
  ),
};

export const Positions: Story = {
  render: () => (
    <div className="grid grid-cols-2 gap-8 p-8">
      <div className="space-y-4">
        <h4 className="text-sm font-medium">Top</h4>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="outline">Top</Button>
          </TooltipTrigger>
          <TooltipContent side="top">
            <p>Tooltip appears above</p>
          </TooltipContent>
        </Tooltip>
      </div>

      <div className="space-y-4">
        <h4 className="text-sm font-medium">Right</h4>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="outline">Right</Button>
          </TooltipTrigger>
          <TooltipContent side="right">
            <p>Tooltip appears to the right</p>
          </TooltipContent>
        </Tooltip>
      </div>

      <div className="space-y-4">
        <h4 className="text-sm font-medium">Bottom</h4>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="outline">Bottom</Button>
          </TooltipTrigger>
          <TooltipContent side="bottom">
            <p>Tooltip appears below</p>
          </TooltipContent>
        </Tooltip>
      </div>

      <div className="space-y-4">
        <h4 className="text-sm font-medium">Left</h4>
        <Tooltip>
          <TooltipTrigger asChild>
            <Button variant="outline">Left</Button>
          </TooltipTrigger>
          <TooltipContent side="left">
            <p>Tooltip appears to the left</p>
          </TooltipContent>
        </Tooltip>
      </div>
    </div>
  ),
};

export const DocumentationHelper: Story = {
  render: () => (
    <div className="space-y-4 p-4">
      <div className="flex items-center gap-2">
        <span className="text-sm">Configuration Schema</span>
        <Tooltip>
          <TooltipTrigger asChild>
            <HelpCircle className="h-4 w-4 text-muted-foreground cursor-help" />
          </TooltipTrigger>
          <TooltipContent className="max-w-sm">
            <p>
              The configuration schema defines the structure and validation rules for your project
              settings. It ensures all required fields are present and properly formatted.
            </p>
          </TooltipContent>
        </Tooltip>
      </div>

      <div className="flex items-center gap-2">
        <span className="text-sm">Runtime Permissions</span>
        <Tooltip>
          <TooltipTrigger asChild>
            <Info className="h-4 w-4 text-blue-500 cursor-help" />
          </TooltipTrigger>
          <TooltipContent className="max-w-sm">
            <p>
              Runtime permissions control what your tools can access during execution. Common
              permissions include --allow-read, --allow-net, and --allow-env for file system,
              network, and environment access.
            </p>
          </TooltipContent>
        </Tooltip>
      </div>

      <div className="flex items-center gap-2">
        <span className="text-sm">Advanced Settings</span>
        <Tooltip>
          <TooltipTrigger asChild>
            <Settings className="h-4 w-4 text-gray-500 cursor-help" />
          </TooltipTrigger>
          <TooltipContent>
            <p>Configure advanced options for expert users</p>
          </TooltipContent>
        </Tooltip>
      </div>
    </div>
  ),
};
