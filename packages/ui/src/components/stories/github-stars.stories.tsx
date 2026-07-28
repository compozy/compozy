import type { Meta, StoryObj } from "@storybook/react-vite";
import { Star } from "lucide-react";

import { GithubLogo } from "../../logos";
import {
  GithubStars,
  GithubStarsIcon,
  GithubStarsNumber,
  GithubStarsParticles,
} from "../animation/github-stars";
import { Button } from "../button";

interface GithubStarButtonDemoProps {
  value: number;
  thousandSeparator?: string;
}

function GithubStarButtonDemo({ value, thousandSeparator }: GithubStarButtonDemoProps) {
  return (
    <GithubStars value={value}>
      <Button
        variant="outline"
        className="gap-2 rounded-full border-line text-muted hover:bg-hover hover:text-fg"
      >
        <GithubLogo aria-hidden className="size-4" />
        <span>Star on GitHub</span>
        <GithubStarsNumber thousandSeparator={thousandSeparator} />
        <GithubStarsParticles>
          <GithubStarsIcon
            icon={Star}
            className="h-4 w-4"
            color="#D6A647"
            activeClassName="[fill:#D6A647] [color:#D6A647]"
          />
        </GithubStarsParticles>
      </Button>
    </GithubStars>
  );
}

const meta: Meta<typeof GithubStarButtonDemo> = {
  title: "components/ui/GithubStars",
  component: GithubStarButtonDemo,
  parameters: {
    layout: "centered",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const StarButton: Story = {
  args: { value: 1280 },
};

export const LargeCount: Story = {
  args: { value: 42567, thousandSeparator: "," },
};
