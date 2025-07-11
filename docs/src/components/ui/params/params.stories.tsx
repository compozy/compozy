import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import {
  Param,
  ParamBody,
  ParamCollapse,
  ParamCollapseItem,
  Params,
  ParamsBody,
  ParamsHeader,
} from "./index";

const meta: Meta<typeof Params> = {
  title: "UI/Params/Params",
  component: Params,
  parameters: {
    layout: "padded",
  },
  decorators: [
    Story => (
      <div style={{ maxWidth: 700 }}>
        <Story />
      </div>
    ),
  ],
  tags: ["autodocs"],
  argTypes: {
    collapsible: {
      control: "boolean",
      description: "Whether the params should be collapsible",
    },
    scrollable: {
      control: "boolean",
      description: "Whether the params content should be scrollable",
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    children: (
      <>
        <ParamsHeader>
          <h3 className="text-lg font-semibold">Request Parameters</h3>
          <p className="text-sm text-muted-foreground">Configuration options for the API request</p>
        </ParamsHeader>
        <ParamsBody>
          <Param path="api_key" paramType="header" type="string" required>
            <ParamBody>Your API key for authentication</ParamBody>
          </Param>
          <Param path="timeout" paramType="query" type="number" default="30">
            <ParamBody>Request timeout in seconds</ParamBody>
          </Param>
          <Param path="retries" paramType="query" type="integer" default="3">
            <ParamBody>Number of retry attempts</ParamBody>
          </Param>
        </ParamsBody>
      </>
    ),
  },
};

export const BasicUsage: Story = {
  render: () => (
    <Params>
      <ParamsHeader>
        <h3 className="text-lg font-semibold">User Information</h3>
        <p className="text-sm text-muted-foreground">Basic user data structure</p>
      </ParamsHeader>
      <ParamsBody>
        <Param path="name" paramType="body" type="string" required>
          <ParamBody>User's full name</ParamBody>
        </Param>
        <Param path="email" paramType="body" type="string" required>
          <ParamBody>User's email address</ParamBody>
        </Param>
        <Param path="age" paramType="body" type="integer">
          <ParamBody>User's age (optional)</ParamBody>
        </Param>
      </ParamsBody>
    </Params>
  ),
};

export const ResponseFormat: Story = {
  render: () => (
    <Params>
      <ParamsHeader>
        <h3 className="text-lg font-semibold">Response Format</h3>
        <p className="text-sm text-muted-foreground">Structure of the API response</p>
      </ParamsHeader>
      <ParamsBody>
        <Param path="success" paramType="response" type="boolean" required>
          <ParamBody>Indicates if the request was successful</ParamBody>
        </Param>
        <Param path="data" paramType="response" type="object">
          <ParamBody>Response data payload</ParamBody>
        </Param>
        <Param path="errors" paramType="response" type="array">
          <ParamBody>Array of error messages if any</ParamBody>
        </Param>
      </ParamsBody>
    </Params>
  ),
};

export const CollapsibleParams: Story = {
  render: () => (
    <Params collapsible defaultOpen>
      <ParamsHeader>
        <h3 className="text-lg font-semibold">Database Configuration</h3>
        <p className="text-sm text-muted-foreground">Connection settings for the database</p>
      </ParamsHeader>
      <ParamsBody>
        <Param path="host" paramType="body" type="string" required>
          <ParamBody>Database host address</ParamBody>
        </Param>
        <Param path="port" paramType="body" type="integer" default="5432">
          <ParamBody>Database port number</ParamBody>
        </Param>
        <Param path="options" paramType="body" type="object">
          <ParamBody>Connection configuration options</ParamBody>
          <ParamCollapse defaultValue="properties">
            <ParamCollapseItem value="properties" title="Properties">
              <Param path="ssl" type="boolean" default="true">
                <ParamBody>Enable SSL connection</ParamBody>
              </Param>
              <Param path="timeout" type="integer" default="5000">
                <ParamBody>Connection timeout in milliseconds</ParamBody>
              </Param>
              <Param path="pool_size" type="integer" default="10">
                <ParamBody>Maximum number of connections in pool</ParamBody>
              </Param>
            </ParamCollapseItem>
          </ParamCollapse>
        </Param>
      </ParamsBody>
    </Params>
  ),
};

export const QueryAndPathParams: Story = {
  render: () => (
    <Params>
      <ParamsHeader>
        <h3 className="text-lg font-semibold">Endpoint Parameters</h3>
      </ParamsHeader>
      <ParamsBody>
        <Param path="userId" paramType="path" type="string" required>
          <ParamBody>Unique identifier</ParamBody>
        </Param>
        <Param path="format" paramType="query" type="string" default="json">
          <ParamBody>Response format</ParamBody>
        </Param>
      </ParamsBody>
    </Params>
  ),
};

export const LongContentExample: Story = {
  render: () => (
    <Params>
      <ParamsHeader>
        <h3 className="text-lg font-semibold">Complex Object Structure</h3>
        <p className="text-sm text-muted-foreground">
          Detailed parameter documentation with extensive descriptions
        </p>
      </ParamsHeader>
      <ParamsBody>
        <Param path="configuration" paramType="body" type="object" required>
          <ParamBody>
            <p>
              A comprehensive configuration object that controls various aspects of the application
              behavior. This object contains multiple nested properties that allow fine-grained
              control over different subsystems.
            </p>
          </ParamBody>
        </Param>
      </ParamsBody>
    </Params>
  ),
};

export const MixedParameterTypes: Story = {
  render: () => (
    <Params>
      <ParamsHeader>
        <h3 className="text-lg font-semibold">Update User Endpoint</h3>
        <p className="text-sm text-muted-foreground">
          Complete parameter documentation for user update operation
        </p>
      </ParamsHeader>
      <ParamsBody>
        <Param path="userId" paramType="path" type="string" required>
          <ParamBody>User ID from URL path</ParamBody>
        </Param>
        <Param path="fields" paramType="query" type="string">
          <ParamBody>Fields to include in response</ParamBody>
        </Param>
        <Param path="Authorization" paramType="header" type="string" required>
          <ParamBody>Bearer token for authentication</ParamBody>
        </Param>
        <Param path="user" paramType="body" type="object" required>
          <ParamBody>User data to update</ParamBody>
        </Param>
      </ParamsBody>
    </Params>
  ),
};

export const ScrollableContent: Story = {
  render: () => (
    <Params>
      <ParamsHeader>
        <h3 className="text-lg font-semibold">Large Parameter Set</h3>
        <p className="text-sm text-muted-foreground">
          Example with many parameters that require scrolling
        </p>
      </ParamsHeader>
      <ParamsBody>
        {Array.from({ length: 15 }, (_, i) => (
          <Param key={i} path={`param_${i + 1}`} paramType="body" type="string">
            <ParamBody>
              This is parameter number {i + 1} with a detailed description that explains its purpose
              and usage within the API.
            </ParamBody>
          </Param>
        ))}
      </ParamsBody>
    </Params>
  ),
};

export const WithScrollableBody: Story = {
  render: () => (
    <Params>
      <ParamsHeader>
        <h3 className="text-lg font-semibold">Scrollable Parameter List</h3>
        <p className="text-sm text-muted-foreground">Parameters with scrollable content area</p>
      </ParamsHeader>
      <ParamsBody scrollable={true}>
        {Array.from({ length: 20 }, (_, i) => (
          <Param key={i} path={`config_${i + 1}`} paramType="body" type="string">
            <ParamBody>Configuration parameter {i + 1}</ParamBody>
          </Param>
        ))}
      </ParamsBody>
    </Params>
  ),
};
