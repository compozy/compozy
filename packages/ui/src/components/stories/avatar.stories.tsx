import type { Meta, StoryObj } from "@storybook/react-vite";
import { CircleCheckIcon } from "lucide-react";

import { Avatar, AvatarBadge, AvatarFallback, AvatarGroup, AvatarImage } from "../avatar";

const meta: Meta<typeof Avatar> = {
  title: "components/ui/Avatar",
  component: Avatar,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "User avatar with image + fallback slots. Use AvatarGroup to overlap multiple participants and AvatarBadge for presence cues.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const AVATAR_IMAGE_SRC =
  "data:image/svg+xml;utf8," +
  encodeURIComponent(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 40"><rect width="40" height="40" fill="%23E8572A"/><text x="50%" y="55%" dominant-baseline="middle" text-anchor="middle" fill="white" font-family="system-ui" font-size="16" font-weight="600">PN</text></svg>'
  );

const BROKEN_IMAGE_SRC = "https://example.invalid/avatar-does-not-exist.png";

export const WithImage: Story = {
  args: {},
  render: () => (
    <Avatar>
      <AvatarImage src={AVATAR_IMAGE_SRC} alt="Pedro Nauck" />
      <AvatarFallback>PN</AvatarFallback>
    </Avatar>
  ),
};

export const FallbackInitials: Story = {
  args: {},
  parameters: {
    docs: {
      description: {
        story:
          "Uses a URL that intentionally fails to load so the fallback initials render in its place.",
      },
    },
  },
  render: () => (
    <Avatar>
      <AvatarImage src={BROKEN_IMAGE_SRC} alt="Alex Rivera" />
      <AvatarFallback>AR</AvatarFallback>
    </Avatar>
  ),
};

export const WithBadge: Story = {
  args: {},
  render: () => (
    <Avatar size="lg">
      <AvatarImage src={AVATAR_IMAGE_SRC} alt="Pedro Nauck" />
      <AvatarFallback>PN</AvatarFallback>
      <AvatarBadge>
        <CircleCheckIcon />
      </AvatarBadge>
    </Avatar>
  ),
};

export const Group: Story = {
  args: {},
  render: () => (
    <AvatarGroup>
      <Avatar>
        <AvatarImage src={AVATAR_IMAGE_SRC} alt="Pedro Nauck" />
        <AvatarFallback>PN</AvatarFallback>
      </Avatar>
      <Avatar>
        <AvatarFallback>AR</AvatarFallback>
      </Avatar>
      <Avatar>
        <AvatarFallback>MK</AvatarFallback>
      </Avatar>
    </AvatarGroup>
  ),
};
