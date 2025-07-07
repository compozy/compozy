import type { Meta, StoryObj } from "@storybook/react";
import { SchemaParams } from "./schema-params";

// Import actual schemas from the schemas folder
import agentSchema from "../../../../schemas/agent.json";
import memorySchema from "../../../../schemas/memory.json";
import taskSchema from "../../../../schemas/task.json";
import toolSchema from "../../../../schemas/tool.json";

const meta: Meta<typeof SchemaParams> = {
  title: "UI/Params/SchemaParams",
  component: SchemaParams,
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
      description: "The type of parameter styling",
    },
    rootPath: {
      control: "text",
      description: "Root path for the schema",
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

// Simple schema example
const simpleSchema = {
  type: "object",
  properties: {
    name: {
      type: "string",
      description: "The user's full name",
    },
    email: {
      type: "string",
      format: "email",
      description: "The user's email address",
    },
    age: {
      type: "integer",
      minimum: 0,
      maximum: 120,
      description: "The user's age in years",
    },
  },
  required: ["name", "email"],
};

export const SimpleObject: Story = {
  args: {
    schema: simpleSchema,
    paramType: "body",
  },
};

export const SimpleObjectCollapsible: Story = {
  args: {
    schema: simpleSchema,
    paramType: "body",
    title: "User Information",
    defaultExpanded: true,
  },
};

// Nested object with arrays
const nestedSchema = {
  type: "object",
  properties: {
    user: {
      type: "object",
      description: "User information",
      properties: {
        id: {
          type: "string",
          format: "uuid",
          description: "Unique user identifier",
        },
        profile: {
          type: "object",
          description: "User profile data",
          properties: {
            firstName: {
              type: "string",
              description: "User's first name",
            },
            lastName: {
              type: "string",
              description: "User's last name",
            },
            avatar: {
              type: "string",
              format: "uri",
              description: "URL to user's avatar image",
            },
          },
          required: ["firstName", "lastName"],
        },
        roles: {
          type: "array",
          description: "User roles in the system",
          items: {
            type: "string",
            enum: ["admin", "user", "moderator"],
          },
        },
      },
      required: ["id", "profile"],
    },
    settings: {
      type: "object",
      description: "Application settings",
      properties: {
        theme: {
          type: "string",
          enum: ["light", "dark", "auto"],
          default: "auto",
          description: "UI theme preference",
        },
        notifications: {
          type: "boolean",
          default: true,
          description: "Enable push notifications",
        },
      },
    },
  },
  required: ["user"],
};

export const NestedObjects: Story = {
  args: {
    schema: nestedSchema,
    paramType: "body",
  },
};

// Schema with anyOf/oneOf
const combinatorialSchema = {
  type: "object",
  properties: {
    payment: {
      description: "Payment method details",
      oneOf: [
        {
          type: "object",
          properties: {
            type: {
              type: "string",
              const: "credit_card",
            },
            cardNumber: {
              type: "string",
              pattern: "^[0-9]{16}$",
              description: "16-digit card number",
            },
            cvv: {
              type: "string",
              pattern: "^[0-9]{3,4}$",
              description: "Card verification value",
            },
          },
          required: ["type", "cardNumber", "cvv"],
        },
        {
          type: "object",
          properties: {
            type: {
              type: "string",
              const: "paypal",
            },
            email: {
              type: "string",
              format: "email",
              description: "PayPal account email",
            },
          },
          required: ["type", "email"],
        },
        {
          type: "object",
          properties: {
            type: {
              type: "string",
              const: "bank_transfer",
            },
            accountNumber: {
              type: "string",
              description: "Bank account number",
            },
            routingNumber: {
              type: "string",
              description: "Bank routing number",
            },
          },
          required: ["type", "accountNumber", "routingNumber"],
        },
      ],
    },
  },
  required: ["payment"],
};

export const WithOneOf: Story = {
  args: {
    schema: combinatorialSchema,
    paramType: "body",
  },
};

// Array with complex items
const arraySchema = {
  type: "object",
  properties: {
    products: {
      type: "array",
      description: "List of products in the shopping cart",
      minItems: 1,
      items: {
        type: "object",
        properties: {
          id: {
            type: "string",
            description: "Product SKU",
          },
          name: {
            type: "string",
            description: "Product name",
          },
          price: {
            type: "number",
            minimum: 0,
            description: "Product price in USD",
          },
          quantity: {
            type: "integer",
            minimum: 1,
            default: 1,
            description: "Quantity to purchase",
          },
          tags: {
            type: "array",
            description: "Product categories/tags",
            items: {
              type: "string",
            },
          },
        },
        required: ["id", "name", "price"],
      },
    },
    coupon: {
      type: "string",
      pattern: "^[A-Z0-9]{5,10}$",
      description: "Discount coupon code",
    },
  },
  required: ["products"],
};

export const WithArrays: Story = {
  args: {
    schema: arraySchema,
    paramType: "body",
  },
};

// Real schema from task.json
export const TaskSchema: Story = {
  name: "Task Schema (Real)",
  args: {
    schema: taskSchema,
    paramType: "body",
    title: "Task Configuration Schema",
    defaultExpanded: false,
  },
};

// Agent schema example
export const AgentSchema: Story = {
  name: "Agent Schema (Real)",
  args: {
    schema: agentSchema,
    paramType: "body",
    title: "Agent Configuration Schema",
    defaultExpanded: false,
  },
};

// Tool schema example
export const ToolSchema: Story = {
  name: "Tool Schema (Real)",
  args: {
    schema: toolSchema,
    paramType: "body",
    title: "Tool Configuration Schema",
    defaultExpanded: false,
  },
};

// Memory schema example
export const MemorySchema: Story = {
  name: "Memory Schema (Real)",
  args: {
    schema: memorySchema,
    paramType: "body",
    title: "Memory Configuration Schema",
    defaultExpanded: false,
  },
};

// Query parameters example
const querySchema = {
  type: "object",
  properties: {
    page: {
      type: "integer",
      minimum: 1,
      default: 1,
      description: "Page number for pagination",
    },
    limit: {
      type: "integer",
      minimum: 1,
      maximum: 100,
      default: 20,
      description: "Number of items per page",
    },
    sort: {
      type: "string",
      enum: ["asc", "desc"],
      default: "asc",
      description: "Sort order",
    },
    filter: {
      type: "string",
      description: "Filter query string",
    },
  },
};

export const QueryParameters: Story = {
  args: {
    schema: querySchema,
    paramType: "query",
  },
};

// Response schema example
const responseSchema = {
  type: "object",
  properties: {
    success: {
      type: "boolean",
      description: "Whether the request was successful",
    },
    data: {
      type: "object",
      description: "Response data",
      properties: {
        id: {
          type: "string",
          description: "Resource identifier",
        },
        createdAt: {
          type: "string",
          format: "date-time",
          description: "Creation timestamp",
        },
        updatedAt: {
          type: "string",
          format: "date-time",
          description: "Last update timestamp",
        },
      },
      required: ["id", "createdAt"],
    },
    error: {
      type: "object",
      description: "Error details (only present when success is false)",
      properties: {
        code: {
          type: "string",
          description: "Error code",
        },
        message: {
          type: "string",
          description: "Human-readable error message",
        },
      },
    },
  },
  required: ["success"],
};

export const ResponseSchema: Story = {
  args: {
    schema: responseSchema,
    paramType: "response",
  },
};

// Schema with allOf (inheritance)
const allOfSchema = {
  type: "object",
  properties: {
    employee: {
      description: "Employee record",
      allOf: [
        {
          type: "object",
          properties: {
            name: {
              type: "string",
              description: "Full name",
            },
            email: {
              type: "string",
              format: "email",
              description: "Email address",
            },
          },
          required: ["name", "email"],
        },
        {
          type: "object",
          properties: {
            employeeId: {
              type: "string",
              description: "Employee ID",
            },
            department: {
              type: "string",
              description: "Department name",
            },
            salary: {
              type: "number",
              minimum: 0,
              description: "Annual salary",
            },
          },
          required: ["employeeId", "department"],
        },
      ],
    },
  },
};

export const WithAllOf: Story = {
  args: {
    schema: allOfSchema,
    paramType: "body",
  },
};
