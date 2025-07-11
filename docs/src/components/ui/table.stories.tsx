import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Badge } from "./badge";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "./table";

const meta = {
  title: "UI/Table",
  component: Table,
  parameters: {
    layout: "centered",
  },
  tags: ["autodocs"],
  argTypes: {
    className: {
      control: "text",
      description: "Additional CSS classes to apply",
    },
  },
} satisfies Meta<typeof Table>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <Table className="w-full max-w-2xl">
      <TableCaption>A list of your recent invoices.</TableCaption>
      <TableHeader>
        <TableRow>
          <TableHead className="w-[100px]">Invoice</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Method</TableHead>
          <TableHead className="text-right">Amount</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow>
          <TableCell className="font-medium">INV001</TableCell>
          <TableCell>Paid</TableCell>
          <TableCell>Credit Card</TableCell>
          <TableCell className="text-right">$250.00</TableCell>
        </TableRow>
        <TableRow>
          <TableCell className="font-medium">INV002</TableCell>
          <TableCell>Pending</TableCell>
          <TableCell>PayPal</TableCell>
          <TableCell className="text-right">$150.00</TableCell>
        </TableRow>
        <TableRow>
          <TableCell className="font-medium">INV003</TableCell>
          <TableCell>Unpaid</TableCell>
          <TableCell>Bank Transfer</TableCell>
          <TableCell className="text-right">$350.00</TableCell>
        </TableRow>
        <TableRow>
          <TableCell className="font-medium">INV004</TableCell>
          <TableCell>Paid</TableCell>
          <TableCell>Credit Card</TableCell>
          <TableCell className="text-right">$450.00</TableCell>
        </TableRow>
        <TableRow>
          <TableCell className="font-medium">INV005</TableCell>
          <TableCell>Paid</TableCell>
          <TableCell>PayPal</TableCell>
          <TableCell className="text-right">$550.00</TableCell>
        </TableRow>
        <TableRow>
          <TableCell className="font-medium">INV006</TableCell>
          <TableCell>Pending</TableCell>
          <TableCell>Bank Transfer</TableCell>
          <TableCell className="text-right">$200.00</TableCell>
        </TableRow>
        <TableRow>
          <TableCell className="font-medium">INV007</TableCell>
          <TableCell>Unpaid</TableCell>
          <TableCell>Credit Card</TableCell>
          <TableCell className="text-right">$300.00</TableCell>
        </TableRow>
      </TableBody>
      <TableFooter>
        <TableRow>
          <TableCell colSpan={3}>Total</TableCell>
          <TableCell className="text-right">$2,250.00</TableCell>
        </TableRow>
      </TableFooter>
    </Table>
  ),
};

export const WithBadges: Story = {
  render: () => (
    <Table className="w-full max-w-3xl">
      <TableCaption>API endpoints and their current status.</TableCaption>
      <TableHeader>
        <TableRow>
          <TableHead>Endpoint</TableHead>
          <TableHead>Method</TableHead>
          <TableHead>Status</TableHead>
          <TableHead>Rate Limit</TableHead>
          <TableHead className="text-right">Response Time</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow>
          <TableCell className="font-medium">/api/users</TableCell>
          <TableCell>GET</TableCell>
          <TableCell>
            <Badge variant="success">Active</Badge>
          </TableCell>
          <TableCell>1000/hour</TableCell>
          <TableCell className="text-right">120ms</TableCell>
        </TableRow>
        <TableRow>
          <TableCell className="font-medium">/api/users</TableCell>
          <TableCell>POST</TableCell>
          <TableCell>
            <Badge variant="success">Active</Badge>
          </TableCell>
          <TableCell>100/hour</TableCell>
          <TableCell className="text-right">85ms</TableCell>
        </TableRow>
        <TableRow>
          <TableCell className="font-medium">/api/auth/login</TableCell>
          <TableCell>POST</TableCell>
          <TableCell>
            <Badge variant="warning">Deprecated</Badge>
          </TableCell>
          <TableCell>50/hour</TableCell>
          <TableCell className="text-right">200ms</TableCell>
        </TableRow>
        <TableRow>
          <TableCell className="font-medium">/api/data/export</TableCell>
          <TableCell>GET</TableCell>
          <TableCell>
            <Badge variant="destructive">Offline</Badge>
          </TableCell>
          <TableCell>10/hour</TableCell>
          <TableCell className="text-right">—</TableCell>
        </TableRow>
        <TableRow>
          <TableCell className="font-medium">/api/webhooks</TableCell>
          <TableCell>POST</TableCell>
          <TableCell>
            <Badge variant="info">Beta</Badge>
          </TableCell>
          <TableCell>500/hour</TableCell>
          <TableCell className="text-right">95ms</TableCell>
        </TableRow>
      </TableBody>
    </Table>
  ),
};

export const Simple: Story = {
  render: () => (
    <Table className="w-full max-w-lg">
      <TableHeader>
        <TableRow>
          <TableHead>Name</TableHead>
          <TableHead>Role</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow>
          <TableCell>John Doe</TableCell>
          <TableCell>Developer</TableCell>
        </TableRow>
        <TableRow>
          <TableCell>Jane Smith</TableCell>
          <TableCell>Designer</TableCell>
        </TableRow>
        <TableRow>
          <TableCell>Bob Johnson</TableCell>
          <TableCell>Manager</TableCell>
        </TableRow>
      </TableBody>
    </Table>
  ),
};

export const ConfigurationTable: Story = {
  render: () => (
    <Table className="w-full max-w-4xl">
      <TableCaption>Compozy configuration options and their descriptions.</TableCaption>
      <TableHeader>
        <TableRow>
          <TableHead>Property</TableHead>
          <TableHead>Type</TableHead>
          <TableHead>Default</TableHead>
          <TableHead>Required</TableHead>
          <TableHead>Description</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow>
          <TableCell className="font-medium">name</TableCell>
          <TableCell>
            <code className="text-sm bg-muted px-1 py-0.5 rounded">string</code>
          </TableCell>
          <TableCell>—</TableCell>
          <TableCell>
            <Badge variant="destructive" size="sm">
              Yes
            </Badge>
          </TableCell>
          <TableCell>The name of your project</TableCell>
        </TableRow>
        <TableRow>
          <TableCell className="font-medium">version</TableCell>
          <TableCell>
            <code className="text-sm bg-muted px-1 py-0.5 rounded">string</code>
          </TableCell>
          <TableCell>—</TableCell>
          <TableCell>
            <Badge variant="destructive" size="sm">
              Yes
            </Badge>
          </TableCell>
          <TableCell>Project version following semver</TableCell>
        </TableRow>
        <TableRow>
          <TableCell className="font-medium">description</TableCell>
          <TableCell>
            <code className="text-sm bg-muted px-1 py-0.5 rounded">string</code>
          </TableCell>
          <TableCell>—</TableCell>
          <TableCell>
            <Badge variant="outline" size="sm">
              No
            </Badge>
          </TableCell>
          <TableCell>Brief description of your project</TableCell>
        </TableRow>
        <TableRow>
          <TableCell className="font-medium">runtime.permissions</TableCell>
          <TableCell>
            <code className="text-sm bg-muted px-1 py-0.5 rounded">string[]</code>
          </TableCell>
          <TableCell>[]</TableCell>
          <TableCell>
            <Badge variant="outline" size="sm">
              No
            </Badge>
          </TableCell>
          <TableCell>Bun runtime permissions for tools</TableCell>
        </TableRow>
      </TableBody>
    </Table>
  ),
};
