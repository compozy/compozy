import type { Meta, StoryObj } from "@storybook/react-vite";
import { KeyRound } from "lucide-react";

import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "../../table";
import { DataSurface, type DataSurfaceState } from "../data-surface";

const meta: Meta<typeof DataSurface> = {
  title: "components/custom/DataSurface",
  component: DataSurface,
  parameters: {
    layout: "padded",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function Surface({ state }: { state: DataSurfaceState }) {
  return (
    <DataSurface state={state} className="w-full max-w-3xl">
      <DataSurface.Loading label="Loading records" />
      <DataSurface.Error
        icon={KeyRound}
        title="Unable to load records"
        description="The daemon returned 503."
      />
      <DataSurface.Empty
        icon={KeyRound}
        title="No records"
        description="Records appear here after the daemon returns metadata."
      />
      <DataSurface.Content className="overflow-hidden rounded-lg border border-line">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Ref</TableHead>
              <TableHead>Namespace</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            <TableRow>
              <TableCell className="font-mono">vault:providers/codex/api_key</TableCell>
              <TableCell>providers</TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </DataSurface.Content>
    </DataSurface>
  );
}

export const Loading: Story = {
  args: {
    state: "loading",
  },
  render: args => <Surface state={args.state} />,
};

export const Error: Story = {
  args: {
    state: "error",
  },
  render: args => <Surface state={args.state} />,
};

export const Empty: Story = {
  args: {
    state: "empty",
  },
  render: args => <Surface state={args.state} />,
};

export const Ready: Story = {
  args: {
    state: "ready",
  },
  render: args => <Surface state={args.state} />,
};
