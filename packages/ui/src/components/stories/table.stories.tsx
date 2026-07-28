import type { Meta, StoryObj } from "@storybook/react-vite";

import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "../table";

const meta: Meta<typeof Table> = {
  title: "components/ui/Table",
  component: Table,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: "Data table with header, body, footer, and caption sub-components.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[640px] bg-background p-4 text-foreground">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

const rows = [
  { id: "sess_01", agent: "claude-code", status: "running", duration: "2m 14s" },
  { id: "sess_02", agent: "codex", status: "paused", duration: "0m 42s" },
  { id: "sess_03", agent: "gemini-cli", status: "completed", duration: "6m 03s" },
];

export const Default: Story = {
  args: {},
  render: () => (
    <Table>
      <TableCaption>Recent agent sessions</TableCaption>
      <TableHeader>
        <TableRow>
          <TableHead>ID</TableHead>
          <TableHead>Agent</TableHead>
          <TableHead>Status</TableHead>
          <TableHead className="text-right">Duration</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map(row => (
          <TableRow key={row.id}>
            <TableCell className="font-mono">{row.id}</TableCell>
            <TableCell>{row.agent}</TableCell>
            <TableCell className="text-muted-foreground">{row.status}</TableCell>
            <TableCell className="text-right tabular-nums">{row.duration}</TableCell>
          </TableRow>
        ))}
      </TableBody>
      <TableFooter>
        <TableRow>
          <TableCell colSpan={3}>Total</TableCell>
          <TableCell className="text-right tabular-nums">8m 59s</TableCell>
        </TableRow>
      </TableFooter>
    </Table>
  ),
};

export const Empty: Story = {
  args: {},
  render: () => (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>ID</TableHead>
          <TableHead>Agent</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        <TableRow>
          <TableCell className="py-8 text-center text-muted-foreground" colSpan={2}>
            No sessions yet.
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
  ),
};
