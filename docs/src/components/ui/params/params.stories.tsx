import type { Meta, StoryObj } from "@storybook/react";
import { FileJson, Settings, User } from "lucide-react";
import { Param } from "./param";
import { Params } from "./params";

const meta: Meta<typeof Params> = {
  title: "UI/Params/Params",
  component: Params,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "A card-like wrapper component for parameters with optional collapsible functionality. Use Params.Header for titles and Params.Body for content.",
      },
    },
  },
  tags: ["autodocs"],
  decorators: [
    Story => (
      <div style={{ maxWidth: 700 }}>
        <Story />
      </div>
    ),
  ],
  argTypes: {
    collapsible: {
      control: { type: "boolean" },
      description: "Whether the params container should be collapsible",
    },
    scrollable: {
      control: { type: "boolean" },
      description: "Whether the params content should be scrollable (default: false)",
    },
    defaultOpen: {
      control: { type: "boolean" },
      description: "Whether the collapsible should be open by default",
    },
    className: {
      control: { type: "text" },
      description: "Additional CSS classes",
    },
  },
};

export default meta;
type Story = StoryObj<typeof Params>;

export const Default: Story = {
  args: {
    collapsible: false,
    scrollable: false,
  },
  render: args => (
    <Params {...args}>
      <Params.Header>
        <Settings className="size-4" />
        <span>Configuration Parameters (No Scroll)</span>
      </Params.Header>
      <Params.Body>
        <Param path="apiKey" type="string" required paramType="body">
          <Param.Body>Your API key for authentication</Param.Body>
        </Param>
        <Param path="timeout" type="number" default="30" paramType="body">
          <Param.Body>Request timeout in seconds</Param.Body>
        </Param>
        <Param path="retries" type="number" default="3" paramType="body">
          <Param.Body>Number of retry attempts</Param.Body>
        </Param>
      </Params.Body>
    </Params>
  ),
};

export const Collapsible: Story = {
  args: {
    collapsible: true,
    defaultOpen: true,
  },
  render: args => (
    <Params {...args}>
      <Params.Header>
        <User className="size-4" />
        <span>User Parameters</span>
      </Params.Header>
      <Params.Body>
        <Param path="name" type="string" required paramType="body">
          <Param.Body>User's full name</Param.Body>
        </Param>
        <Param path="email" type="string" required paramType="body">
          <Param.Body>User's email address</Param.Body>
        </Param>
        <Param path="age" type="number" paramType="body">
          <Param.Body>User's age (optional)</Param.Body>
        </Param>
      </Params.Body>
    </Params>
  ),
};

export const CollapsibleClosed: Story = {
  args: {
    collapsible: true,
    defaultOpen: false,
  },
  render: args => (
    <Params {...args}>
      <Params.Header>
        <FileJson className="size-4" />
        <span>API Response Schema</span>
      </Params.Header>
      <Params.Body>
        <Param path="success" type="boolean" required paramType="response">
          <Param.Body>Indicates if the request was successful</Param.Body>
        </Param>
        <Param path="data" type="object" paramType="response">
          <Param.Body>Response data payload</Param.Body>
        </Param>
        <Param path="errors" type="array" paramType="response">
          <Param.Body>Array of error messages if any</Param.Body>
        </Param>
      </Params.Body>
    </Params>
  ),
};

export const WithNestedParams: Story = {
  args: {
    collapsible: true,
    defaultOpen: true,
  },
  render: args => (
    <Params {...args}>
      <Params.Header>
        <Settings className="size-4" />
        <span>Database Configuration</span>
      </Params.Header>
      <Params.Body>
        <Param path="host" type="string" required paramType="body">
          <Param.Body>Database host address</Param.Body>
        </Param>
        <Param path="port" type="number" default="5432" paramType="body">
          <Param.Body>Database port number</Param.Body>
        </Param>
        <Param path="connection" type="object" paramType="body">
          <Param.Body>Connection configuration options</Param.Body>
          <Param.ExpandableRoot defaultValue="properties">
            <Param.ExpandableItem value="properties" title="Properties">
              <Param path="ssl" type="boolean" default="true" paramType="body">
                <Param.Body>Enable SSL connection</Param.Body>
              </Param>
              <Param path="timeout" type="number" default="10000" paramType="body">
                <Param.Body>Connection timeout in milliseconds</Param.Body>
              </Param>
              <Param path="poolSize" type="number" default="10" paramType="body">
                <Param.Body>Maximum number of connections in pool</Param.Body>
              </Param>
            </Param.ExpandableItem>
          </Param.ExpandableRoot>
        </Param>
      </Params.Body>
    </Params>
  ),
};

export const MinimalHeader: Story = {
  args: {
    collapsible: false,
  },
  render: args => (
    <Params {...args}>
      <Params.Header>
        <span>Simple Parameters</span>
      </Params.Header>
      <Params.Body>
        <Param path="id" type="string" required paramType="path">
          <Param.Body>Unique identifier</Param.Body>
        </Param>
        <Param path="format" type="string" default="json" paramType="query">
          <Param.Body>Response format</Param.Body>
        </Param>
      </Params.Body>
    </Params>
  ),
};

export const EmptyState: Story = {
  args: {
    collapsible: true,
    defaultOpen: true,
  },
  render: args => (
    <Params {...args}>
      <Params.Header>
        <FileJson className="size-4" />
        <span>No Parameters</span>
      </Params.Header>
      <Params.Body>
        <div className="text-sm text-muted-foreground py-4 text-center">
          No parameters defined for this endpoint
        </div>
      </Params.Body>
    </Params>
  ),
};

export const DifferentParamTypes: Story = {
  args: {
    collapsible: true,
    defaultOpen: true,
  },
  render: args => (
    <Params {...args}>
      <Params.Header>
        <Settings className="size-4" />
        <span>Mixed Parameter Types</span>
      </Params.Header>
      <Params.Body>
        <Param path="userId" type="string" required paramType="path">
          <Param.Body>User ID from URL path</Param.Body>
        </Param>
        <Param path="include" type="string" paramType="query">
          <Param.Body>Fields to include in response</Param.Body>
        </Param>
        <Param path="authorization" type="string" required paramType="header">
          <Param.Body>Bearer token for authentication</Param.Body>
        </Param>
        <Param path="userData" type="object" required paramType="body">
          <Param.Body>User data to update</Param.Body>
        </Param>
      </Params.Body>
    </Params>
  ),
};

export const LargeContentWithScroll: Story = {
  args: {
    collapsible: true,
    defaultOpen: true,
    scrollable: true,
  },
  render: args => (
    <Params {...args}>
      <Params.Header>
        <FileJson className="size-4" />
        <span>Large Schema (With Scrollable Prop)</span>
      </Params.Header>
      <Params.Body>
        {Array.from({ length: 30 }, (_, i) => (
          <Param key={i} path={`field${i + 1}`} type="string" paramType="body">
            <Param.Body>
              This is field {i + 1} with a longer description to demonstrate how the scroll area
              works when content exceeds 500px height. Lorem ipsum dolor sit amet, consectetur
              adipiscing elit.
            </Param.Body>
          </Param>
        ))}
      </Params.Body>
    </Params>
  ),
};

export const LargeContentWithoutScroll: Story = {
  args: {
    collapsible: false,
    scrollable: false,
  },
  render: args => (
    <Params {...args}>
      <Params.Header>
        <FileJson className="size-4" />
        <span>Large Content (No Scroll)</span>
      </Params.Header>
      <Params.Body>
        {Array.from({ length: 10 }, (_, i) => (
          <Param key={i} path={`field${i + 1}`} type="string" paramType="body">
            <Param.Body>
              This is field {i + 1}. Without scrollable prop, content will expand naturally.
            </Param.Body>
          </Param>
        ))}
      </Params.Body>
    </Params>
  ),
};

export const ScrollableOnBody: Story = {
  args: {
    collapsible: false,
    scrollable: false, // Root level is false
  },
  render: args => (
    <Params {...args}>
      <Params.Header>
        <Settings className="size-4" />
        <span>Scrollable Override on Body</span>
      </Params.Header>
      <Params.Body scrollable={true}>
        {" "}
        {/* Override at body level */}
        {Array.from({ length: 20 }, (_, i) => (
          <Param key={i} path={`param${i + 1}`} type="string" paramType="body">
            <Param.Body>
              Parameter {i + 1} - Body has scrollable=true while root has scrollable=false
            </Param.Body>
          </Param>
        ))}
      </Params.Body>
    </Params>
  ),
};
