import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Accordion, AccordionGroup } from "./accordion-group";

const meta: Meta<typeof AccordionGroup> = {
  title: "UI/AccordionGroup",
  component: AccordionGroup,
  parameters: {
    layout: "padded",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <AccordionGroup>
      <Accordion title="🤖 AI Components">
        - **Agent Overview** - Comprehensive guide to AI agents - **Agent Memory** ↔ **Memory
        Systems** - **Agent Tools** ↔ **Tools Overview** - **LLM Integration** ↔ **Provider
        Configuration**
      </Accordion>

      <Accordion title="⚙️ Execution Engine">
        - **All Task Types** - Complete task reference - **Memory Tasks** ↔ **Memory Operations** -
        **Signal Tasks** ↔ **Signal Overview** - **Advanced Patterns** - Complex orchestration
      </Accordion>

      <Accordion title="🔧 Configuration System">
        - **YAML Templates** - Dynamic configuration engine - **Template Variables** - Data access
        patterns - **Workflow Configuration** - Setup orchestration - **Project Setup** - Foundation
        configuration
      </Accordion>
    </AccordionGroup>
  ),
};
