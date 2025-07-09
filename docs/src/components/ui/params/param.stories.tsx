import type { Meta, StoryObj } from "@storybook/react";
import { Param } from "./param";

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
    children: <Param.Body>The unique identifier for the user</Param.Body>,
  },
};

export const WithBodyComponent: Story = {
  render: () => (
    <Param path="userId" paramType="path" type="string" required>
      <Param.Body>
        The unique identifier for the user. This ID is used to fetch user-specific data and must be
        a valid UUID format.
      </Param.Body>
    </Param>
  ),
};

export const WithExpandableProperties: Story = {
  render: () => (
    <Param path="user" paramType="body" type="object" required>
      <Param.Body>User object with nested properties</Param.Body>
      <Param.ExpandableRoot defaultValue="properties">
        <Param.ExpandableItem value="properties" title="Properties">
          <Param path="name" type="string" required>
            <Param.Body>The user's full name</Param.Body>
          </Param>
          <Param path="email" type="string" required>
            <Param.Body>The user's email address</Param.Body>
          </Param>
          <Param path="age" type="integer">
            <Param.Body>The user's age (optional)</Param.Body>
          </Param>
          <Param path="preferences" type="object">
            <Param.Body>User preferences object</Param.Body>
            <Param.ExpandableRoot>
              <Param.ExpandableItem value="prefs" title="Preferences properties">
                <Param path="theme" type="string" default="light">
                  <Param.Body>UI theme preference</Param.Body>
                </Param>
                <Param path="notifications" type="boolean" default="true">
                  <Param.Body>Whether to receive notifications</Param.Body>
                </Param>
              </Param.ExpandableItem>
            </Param.ExpandableRoot>
          </Param>
        </Param.ExpandableItem>
      </Param.ExpandableRoot>
    </Param>
  ),
};

export const NestedObjectExample: Story = {
  render: () => (
    <Param path="config" paramType="body" type="object">
      <Param.Body>Configuration object with multiple nested levels</Param.Body>
      <Param.ExpandableRoot defaultValue="config">
        <Param.ExpandableItem value="config" title="config properties">
          <Param path="api" type="object" required>
            <Param.Body>API configuration settings</Param.Body>
            <Param.ExpandableRoot>
              <Param.ExpandableItem value="api" title="api properties">
                <Param path="baseUrl" type="string" required>
                  <Param.Body>The base URL for API requests</Param.Body>
                </Param>
                <Param path="timeout" type="integer" default="5000">
                  <Param.Body>Request timeout in milliseconds</Param.Body>
                </Param>
                <Param path="retries" type="integer" default="3">
                  <Param.Body>Number of retry attempts</Param.Body>
                </Param>
              </Param.ExpandableItem>
            </Param.ExpandableRoot>
          </Param>
          <Param path="database" type="object">
            <Param.Body>Database configuration</Param.Body>
            <Param.ExpandableRoot>
              <Param.ExpandableItem value="database" title="database properties">
                <Param path="host" type="string" required>
                  <Param.Body>Database host address</Param.Body>
                </Param>
                <Param path="port" type="integer" default="5432">
                  <Param.Body>Database port number</Param.Body>
                </Param>
                <Param path="ssl" type="boolean" default="true">
                  <Param.Body>Whether to use SSL connection</Param.Body>
                </Param>
              </Param.ExpandableItem>
            </Param.ExpandableRoot>
          </Param>
        </Param.ExpandableItem>
      </Param.ExpandableRoot>
    </Param>
  ),
};

export const ArrayWithItems: Story = {
  render: () => (
    <Param path="items" paramType="body" type="array" required>
      <Param.Body>Array of product items</Param.Body>
      <Param.ExpandableRoot defaultValue="array-items">
        <Param.ExpandableItem value="array-items" title="array items">
          <Param path="[0]" type="object">
            <Param.Body>Product item object</Param.Body>
            <Param.ExpandableRoot>
              <Param.ExpandableItem value="item" title="item properties">
                <Param path="id" type="string" required>
                  <Param.Body>Product identifier</Param.Body>
                </Param>
                <Param path="name" type="string" required>
                  <Param.Body>Product name</Param.Body>
                </Param>
                <Param path="price" type="number" required>
                  <Param.Body>Product price in USD</Param.Body>
                </Param>
                <Param path="tags" type="array">
                  <Param.Body>Product tags</Param.Body>
                  <Param.ExpandableRoot>
                    <Param.ExpandableItem value="tags" title="tags items">
                      <Param path="[0]" type="string">
                        <Param.Body>Tag name</Param.Body>
                      </Param>
                    </Param.ExpandableItem>
                  </Param.ExpandableRoot>
                </Param>
              </Param.ExpandableItem>
            </Param.ExpandableRoot>
          </Param>
        </Param.ExpandableItem>
      </Param.ExpandableRoot>
    </Param>
  ),
};

export const WithRichDescription: Story = {
  render: () => (
    <Param path="query" paramType="query" type="string">
      <Param.Body>
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
      </Param.Body>
    </Param>
  ),
};

export const DeprecatedParameter: Story = {
  render: () => (
    <Param path="oldParam" paramType="query" type="string" deprecated>
      <Param.Body>
        <p>⚠️ This parameter is deprecated and will be removed in v2.0</p>
        <p>
          Use <code>newParam</code> instead
        </p>
      </Param.Body>
    </Param>
  ),
};

export const MixedContentExample: Story = {
  render: () => (
    <Param path="user" paramType="body" type="object" required>
      <Param.Body>
        User object with nested properties. This demonstrates how you can mix regular text
        descriptions with more complex content.
      </Param.Body>
      <Param.ExpandableRoot defaultValue="properties">
        <Param.ExpandableItem value="properties" title="Properties">
          <Param path="name" type="string" required>
            <Param.Body>The user's full name</Param.Body>
          </Param>
          <Param path="email" type="string" required>
            <Param.Body>The user's email address</Param.Body>
          </Param>
          <Param path="metadata" type="object">
            <Param.Body>
              <p>Additional metadata about the user:</p>
              <ul>
                <li>Created timestamp</li>
                <li>Last login information</li>
                <li>User preferences</li>
              </ul>
            </Param.Body>
          </Param>
        </Param.ExpandableItem>
      </Param.ExpandableRoot>
    </Param>
  ),
};
