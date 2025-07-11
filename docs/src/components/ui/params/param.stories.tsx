import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Param, ParamBody, ParamCollapse, ParamCollapseItem } from "./index";

const meta: Meta<typeof Param> = {
  title: "UI/Params/Param",
  component: Param,
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
    paramType: {
      control: "select",
      options: ["query", "path", "body", "header", "response"],
      description: "The type of parameter",
    },
    type: {
      control: "select",
      options: ["string", "number", "integer", "boolean", "array", "object"],
      description: "The data type of the parameter",
    },
    required: {
      control: "boolean",
      description: "Whether the parameter is required",
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    path: "userId",
    paramType: "path",
    type: "string",
    required: true,
    children: <ParamBody>The unique identifier for the user</ParamBody>,
  },
};

export const WithBodyComponent: Story = {
  render: () => (
    <Param path="userId" paramType="path" type="string" required>
      <ParamBody>
        The unique identifier for the user. This ID is used to fetch user-specific data and must be
        a valid UUID format.
      </ParamBody>
    </Param>
  ),
};

export const WithExpandableProperties: Story = {
  render: () => (
    <Param path="user" paramType="body" type="object" required>
      <ParamBody>User object with nested properties</ParamBody>
      <ParamCollapse defaultValue="properties">
        <ParamCollapseItem value="properties" title="Properties">
          <Param path="name" type="string" required>
            <ParamBody>The user's full name</ParamBody>
          </Param>
          <Param path="email" type="string" required>
            <ParamBody>The user's email address</ParamBody>
          </Param>
          <Param path="age" type="integer">
            <ParamBody>The user's age (optional)</ParamBody>
          </Param>
          <Param path="preferences" type="object">
            <ParamBody>User preferences object</ParamBody>
            <ParamCollapse>
              <ParamCollapseItem value="prefs" title="Preferences properties">
                <Param path="theme" type="string" default="light">
                  <ParamBody>UI theme preference</ParamBody>
                </Param>
                <Param path="notifications" type="boolean" default="true">
                  <ParamBody>Whether to receive notifications</ParamBody>
                </Param>
              </ParamCollapseItem>
            </ParamCollapse>
          </Param>
        </ParamCollapseItem>
      </ParamCollapse>
    </Param>
  ),
};

export const NestedObjectExample: Story = {
  render: () => (
    <Param path="config" paramType="body" type="object">
      <ParamBody>Configuration object with multiple nested levels</ParamBody>
      <ParamCollapse defaultValue="config">
        <ParamCollapseItem value="config" title="config properties">
          <Param path="api" type="object" required>
            <ParamBody>API configuration settings</ParamBody>
            <ParamCollapse>
              <ParamCollapseItem value="api" title="api properties">
                <Param path="baseUrl" type="string" required>
                  <ParamBody>The base URL for API requests</ParamBody>
                </Param>
                <Param path="timeout" type="integer" default="5000">
                  <ParamBody>Request timeout in milliseconds</ParamBody>
                </Param>
                <Param path="retries" type="integer" default="3">
                  <ParamBody>Number of retry attempts</ParamBody>
                </Param>
              </ParamCollapseItem>
            </ParamCollapse>
          </Param>
          <Param path="database" type="object">
            <ParamBody>Database configuration</ParamBody>
            <ParamCollapse>
              <ParamCollapseItem value="database" title="database properties">
                <Param path="host" type="string" required>
                  <ParamBody>Database host address</ParamBody>
                </Param>
                <Param path="port" type="integer" default="5432">
                  <ParamBody>Database port number</ParamBody>
                </Param>
                <Param path="ssl" type="boolean" default="true">
                  <ParamBody>Whether to use SSL connection</ParamBody>
                </Param>
              </ParamCollapseItem>
            </ParamCollapse>
          </Param>
        </ParamCollapseItem>
      </ParamCollapse>
    </Param>
  ),
};

export const ArrayWithItems: Story = {
  render: () => (
    <Param path="items" paramType="body" type="array" required>
      <ParamBody>Array of product items</ParamBody>
      <ParamCollapse defaultValue="array-items">
        <ParamCollapseItem value="array-items" title="array items">
          <Param path="[0]" type="object">
            <ParamBody>Product item object</ParamBody>
            <ParamCollapse>
              <ParamCollapseItem value="item" title="item properties">
                <Param path="id" type="string" required>
                  <ParamBody>Product identifier</ParamBody>
                </Param>
                <Param path="name" type="string" required>
                  <ParamBody>Product name</ParamBody>
                </Param>
                <Param path="price" type="number" required>
                  <ParamBody>Product price in USD</ParamBody>
                </Param>
                <Param path="tags" type="array">
                  <ParamBody>Product tags</ParamBody>
                  <ParamCollapse>
                    <ParamCollapseItem value="tags" title="tags items">
                      <Param path="[0]" type="string">
                        <ParamBody>Tag name</ParamBody>
                      </Param>
                    </ParamCollapseItem>
                  </ParamCollapse>
                </Param>
              </ParamCollapseItem>
            </ParamCollapse>
          </Param>
        </ParamCollapseItem>
      </ParamCollapse>
    </Param>
  ),
};

export const WithRichDescription: Story = {
  render: () => (
    <Param path="query" paramType="query" type="string">
      <ParamBody>
        <p>Search query parameter with special formatting support:</p>
        <ul>
          <li>
            <code>name:John</code> - Search by name
          </li>
          <li>
            <code>email:@example.com</code> - Search by email domain
          </li>
          <li>
            <code>created:&gt;2023-01-01</code> - Search by creation date
          </li>
        </ul>
        <p>
          <strong>Note:</strong> Multiple filters can be combined with spaces
        </p>
      </ParamBody>
    </Param>
  ),
};

export const DeprecatedParameter: Story = {
  render: () => (
    <Param path="oldParam" paramType="query" type="string" deprecated>
      <ParamBody>
        <p>⚠️ This parameter is deprecated and will be removed in v2.0</p>
        <p>
          Use <code>newParam</code> instead
        </p>
      </ParamBody>
    </Param>
  ),
};

export const MixedContentExample: Story = {
  render: () => (
    <Param path="user" paramType="body" type="object" required>
      <ParamBody>
        User object with nested properties. This demonstrates how you can mix regular text
        descriptions with more complex content.
      </ParamBody>
      <ParamCollapse defaultValue="properties">
        <ParamCollapseItem value="properties" title="Properties">
          <Param path="name" type="string" required>
            <ParamBody>The user's full name</ParamBody>
          </Param>
          <Param path="email" type="string" required>
            <ParamBody>The user's email address</ParamBody>
          </Param>
          <Param path="metadata" type="object">
            <ParamBody>
              <p>Additional metadata about the user:</p>
              <ul>
                <li>Created timestamp</li>
                <li>Last login information</li>
                <li>User preferences</li>
              </ul>
            </ParamBody>
          </Param>
        </ParamCollapseItem>
      </ParamCollapse>
    </Param>
  ),
};
