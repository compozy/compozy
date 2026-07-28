import { ChevronRight } from "lucide-react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { LinkedRecordTable } from "../custom/linked-record-table";
import { Pill } from "../custom/pill";

const meta: Meta<typeof LinkedRecordTable> = {
  title: "components/custom/LinkedRecordTable",
  component: LinkedRecordTable,
  args: {},
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Linked record table for dense related-record panels with status dot, identifier pill, status pill, and open action slots.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const records = [
  { id: "TASK-102", title: "Materialize task ledger", owner: "Codex", updated: "2m ago" },
  { id: "TASK-103", title: "Replay event cursor", owner: "Hermes", updated: "8m ago" },
];

/** Single linked record row. */
export const Single: Story = {
  args: {},
  render: () => (
    <LinkedRecordTable label="Dependencies" className="max-w-3xl">
      <LinkedRecordTable.Body>
        <LinkedRecordTable.Row>
          <LinkedRecordTable.Cell className="w-8 pl-4">
            <Pill.Dot tone="success" />
          </LinkedRecordTable.Cell>
          <LinkedRecordTable.Cell>
            <LinkedRecordTable.Title>
              <span>Materialize task ledger</span>
              <div className="flex flex-wrap items-center gap-1.5">
                <Pill mono>TASK-102</Pill>
                <Pill tone="success">done</Pill>
              </div>
            </LinkedRecordTable.Title>
          </LinkedRecordTable.Cell>
          <LinkedRecordTable.Cell>Codex</LinkedRecordTable.Cell>
          <LinkedRecordTable.Cell>2m ago</LinkedRecordTable.Cell>
          <LinkedRecordTable.OpenCell>
            <Pill.Link href="/tasks/task-102">
              Open <ChevronRight className="size-3" />
            </Pill.Link>
          </LinkedRecordTable.OpenCell>
        </LinkedRecordTable.Row>
      </LinkedRecordTable.Body>
    </LinkedRecordTable>
  ),
};

/** Multiple related records keep spacing and columns stable. */
export const Many: Story = {
  args: {},
  render: () => (
    <LinkedRecordTable label="Child tasks" className="max-w-3xl">
      <LinkedRecordTable.Body>
        {records.map(record => (
          <LinkedRecordTable.Row key={record.id}>
            <LinkedRecordTable.Cell className="w-8 pl-4">
              <Pill.Dot tone="accent" />
            </LinkedRecordTable.Cell>
            <LinkedRecordTable.Cell>
              <LinkedRecordTable.Title>
                <span>{record.title}</span>
                <div className="flex flex-wrap items-center gap-1.5">
                  <Pill mono>{record.id}</Pill>
                  <Pill tone="accent">running</Pill>
                </div>
              </LinkedRecordTable.Title>
            </LinkedRecordTable.Cell>
            <LinkedRecordTable.Cell>{record.owner}</LinkedRecordTable.Cell>
            <LinkedRecordTable.Cell>{record.updated}</LinkedRecordTable.Cell>
            <LinkedRecordTable.OpenCell>
              <Pill.Link href={`/tasks/${record.id}`}>
                Open <ChevronRight className="size-3" />
              </Pill.Link>
            </LinkedRecordTable.OpenCell>
          </LinkedRecordTable.Row>
        ))}
      </LinkedRecordTable.Body>
    </LinkedRecordTable>
  ),
};

/** Empty slot lets the host render its own truthful state. */
export const Empty: Story = {
  args: {},
  render: () => (
    <LinkedRecordTable
      label="Runs"
      className="max-w-3xl"
      empty={<p className="text-small-body text-muted">No runs yet.</p>}
    />
  ),
};
