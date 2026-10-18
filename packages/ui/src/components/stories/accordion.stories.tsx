import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "../accordion";

const meta: Meta<typeof Accordion> = {
  title: "components/ui/Accordion",
  component: Accordion,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Disclosure list backed by Base UI. Use Item + Trigger + Content tuples to expose long-form content in collapsible panels.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const faq = [
  {
    value: "retention",
    question: "How long are session events retained?",
    answer:
      "Events live in the per-session SQLite database until the workspace retention policy purges them. Default is 14 days.",
  },
  {
    value: "retention",
    question: "When does the retention sweep run?",
    answer:
      "The retention sweep runs on a cron plus idle triggers, any quiet window over 30 minutes kicks off a pass.",
  },
  {
    value: "remote",
    question: "Can agents talk across workspaces?",
    answer: "Remote access requires an active Gateway provider and the configured access policy.",
  },
] as const;

export const Default: Story = {
  render: () => (
    <div className="w-lg">
      <Accordion defaultValue={[faq[0].value]}>
        {faq.map(item => (
          <AccordionItem key={item.value} value={item.value}>
            <AccordionTrigger>{item.question}</AccordionTrigger>
            <AccordionContent>{item.answer}</AccordionContent>
          </AccordionItem>
        ))}
      </Accordion>
    </div>
  ),
};

export const MultipleExpansion: Story = {
  parameters: {
    docs: {
      description: {
        story: "`multiple` lets several items stay open at the same time.",
      },
    },
  },
  render: () => (
    <div className="w-lg">
      <Accordion multiple defaultValue={[faq[0].value, faq[1].value]}>
        {faq.map(item => (
          <AccordionItem key={item.value} value={item.value}>
            <AccordionTrigger>{item.question}</AccordionTrigger>
            <AccordionContent>{item.answer}</AccordionContent>
          </AccordionItem>
        ))}
      </Accordion>
    </div>
  ),
};

export const OpensAndCloses: Story = {
  render: () => (
    <div className="w-lg">
      <Accordion>
        {faq.map(item => (
          <AccordionItem key={item.value} value={item.value}>
            <AccordionTrigger>{item.question}</AccordionTrigger>
            <AccordionContent>{item.answer}</AccordionContent>
          </AccordionItem>
        ))}
      </Accordion>
    </div>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const trigger = canvas.getByRole("button", { name: faq[0].question });
    await expect(trigger).toHaveAttribute("aria-expanded", "false");
    await userEvent.click(trigger);
    await waitFor(() => expect(trigger).toHaveAttribute("aria-expanded", "true"));
    await userEvent.click(trigger);
    await waitFor(() => expect(trigger).toHaveAttribute("aria-expanded", "false"));
  },
};
