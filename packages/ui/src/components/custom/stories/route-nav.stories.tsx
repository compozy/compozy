import type { Meta, StoryObj } from "@storybook/react-vite";

import { RouteNav } from "../route-nav";

const meta: Meta<typeof RouteNav> = {
  title: "components/custom/RouteNav",
  component: RouteNav,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          'Peer sister-route navigation for the topbar `nav` zone (after identity in the 44px head). Real links inside a labeled `nav`; the active route carries `aria-current="page"` and gets the elevated segment. Panel `Tabs` and mode `PillGroup` keep their own semantics — this is routes only.',
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => (
    <RouteNav aria-label="Tasks views">
      <RouteNav.Link aria-current="page" href="#list">
        List
      </RouteNav.Link>
      <RouteNav.Link href="#kanban">Kanban</RouteNav.Link>
      <RouteNav.Link href="#dashboard">Dashboard</RouteNav.Link>
      <RouteNav.Link href="#inbox">
        Inbox <RouteNav.Count>2</RouteNav.Count>
      </RouteNav.Link>
    </RouteNav>
  ),
};
