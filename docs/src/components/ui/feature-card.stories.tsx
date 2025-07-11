import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Cloud, Code, Database, Palette, Rocket, Shield, Sparkles, Zap } from "lucide-react";
import { FeatureCard, FeatureCardList } from "./feature-card";

const meta = {
  title: "UI/FeatureCard",
  component: FeatureCard,
  parameters: {
    layout: "centered",
  },
  tags: ["autodocs"],
  argTypes: {
    title: {
      control: "text",
      description: "The title of the feature card",
    },
    description: {
      control: "text",
      description: "The description text for the feature",
    },
    href: {
      control: "text",
      description: "Optional link URL. When provided, the entire card becomes clickable",
    },
    icon: {
      control: false,
      description: "Lucide icon component to display",
    },
    size: {
      control: "select",
      options: ["sm", "default", "lg"],
      description: "Size variant for the card",
    },
    className: {
      control: "text",
      description: "Additional CSS classes to apply",
    },
  },
} satisfies Meta<typeof FeatureCard>;

export default meta;
type Story = StoryObj<typeof meta>;

// Basic card without link
export const Default: Story = {
  args: {
    title: "Lightning Fast",
    description: "Built for speed with optimized performance and minimal bundle size.",
    icon: Zap,
    size: "default",
  },
};

// Card with link
export const WithLink: Story = {
  args: {
    title: "Beautiful Design",
    description: "Crafted with attention to detail and modern design principles.",
    icon: Sparkles,
    href: "/docs/design",
    size: "default",
  },
};

// Card without icon
export const NoIcon: Story = {
  args: {
    title: "Simple Feature",
    description: "Sometimes you don't need an icon. This card shows how it looks without one.",
    size: "default",
  },
};

// Size variants
export const SizeVariants: StoryObj<typeof meta> = {
  args: {
    title: "",
    description: "",
  },
  render: () => (
    <div className="w-full max-w-6xl p-8 space-y-8">
      <div>
        <h3 className="text-lg font-semibold mb-4">Small Size</h3>
        <FeatureCardList cols={3} size="sm">
          <FeatureCard
            title="Compact Feature"
            description="Perfect for when you need to show many features in a limited space."
            icon={Zap}
          />
          <FeatureCard
            title="Efficient Layout"
            description="Smaller padding and typography for dense information display."
            icon={Shield}
          />
          <FeatureCard
            title="Space Saving"
            description="Ideal for dashboards and overview pages."
            icon={Code}
          />
        </FeatureCardList>
      </div>

      <div>
        <h3 className="text-lg font-semibold mb-4">Default Size</h3>
        <FeatureCardList cols={2} size="default">
          <FeatureCard
            title="Balanced Design"
            description="The standard size that works well for most use cases with good readability and visual hierarchy."
            icon={Palette}
          />
          <FeatureCard
            title="Versatile Option"
            description="Perfect balance between information density and visual comfort."
            icon={Rocket}
          />
        </FeatureCardList>
      </div>

      <div>
        <h3 className="text-lg font-semibold mb-4">Large Size</h3>
        <FeatureCardList cols={2} size="lg">
          <FeatureCard
            title="Prominent Display"
            description="Large size for hero sections and important feature highlights. Provides maximum visual impact with generous spacing and larger typography for enhanced readability."
            icon={Cloud}
          />
          <FeatureCard
            title="Premium Feel"
            description="Spacious layout that gives your content room to breathe. Perfect for landing pages and marketing sections where you want to make a strong impression."
            icon={Database}
          />
        </FeatureCardList>
      </div>
    </div>
  ),
};

// Multiple cards in a grid
export const CardGrid: StoryObj<typeof meta> = {
  args: {
    title: "",
    description: "",
  },
  render: () => (
    <div className="w-full max-w-6xl p-8">
      <FeatureCardList cols={3}>
        <FeatureCard
          title="Lightning Fast"
          description="Built for speed with optimized performance and minimal bundle size."
          icon={Zap}
          href="/docs/performance"
        />
        <FeatureCard
          title="Beautiful Design"
          description="Crafted with attention to detail and modern design principles."
          icon={Palette}
          href="/docs/design"
        />
        <FeatureCard
          title="Secure by Default"
          description="Enterprise-grade security with built-in protection against common vulnerabilities."
          icon={Shield}
          href="/docs/security"
        />
        <FeatureCard
          title="Developer First"
          description="Intuitive APIs and comprehensive documentation for developers."
          icon={Code}
        />
        <FeatureCard
          title="Cloud Native"
          description="Built from the ground up for modern cloud environments."
          icon={Cloud}
        />
        <FeatureCard
          title="Scalable Architecture"
          description="Handles everything from small projects to enterprise applications."
          icon={Database}
        />
      </FeatureCardList>
    </div>
  ),
};

// Different column layouts
export const ColumnLayouts: StoryObj<typeof meta> = {
  args: {
    title: "",
    description: "",
  },
  render: () => (
    <div className="w-full max-w-6xl p-8 space-y-8">
      <div>
        <h3 className="text-sm font-medium text-muted-foreground mb-4">2 Columns</h3>
        <FeatureCardList cols={2}>
          <FeatureCard
            title="Feature One"
            description="First feature in a two-column layout."
            icon={Rocket}
          />
          <FeatureCard
            title="Feature Two"
            description="Second feature in a two-column layout."
            icon={Sparkles}
          />
        </FeatureCardList>
      </div>

      <div>
        <h3 className="text-sm font-medium text-muted-foreground mb-4">4 Columns</h3>
        <FeatureCardList cols={4}>
          <FeatureCard title="Compact" description="Works well in narrow columns." icon={Zap} />
          <FeatureCard title="Flexible" description="Adapts to available space." icon={Shield} />
          <FeatureCard title="Responsive" description="Mobile-friendly design." icon={Code} />
          <FeatureCard title="Modern" description="Clean and minimal UI." icon={Palette} />
        </FeatureCardList>
      </div>
    </div>
  ),
};

// Long content example
export const LongContent: Story = {
  args: {
    title: "Comprehensive Feature with Extended Description",
    description:
      "This is a much longer description that demonstrates how the card handles extended text content. The card will automatically expand to accommodate the content while maintaining proper spacing and visual hierarchy. Perfect for when you need to provide more detailed information about a particular feature or capability.",
    icon: Database,
    href: "/docs/comprehensive",
    size: "default",
  },
};

// Theme showcase
export const ThemeShowcase: Story = {
  args: {
    title: "",
    description: "",
  },
  render: () => (
    <div className="w-full max-w-4xl p-8">
      <div className="space-y-8">
        <div>
          <p className="text-sm text-muted-foreground mb-4">
            The cards adapt to your theme automatically. Try switching between light and dark modes
            to see the effect.
          </p>
          <FeatureCardList cols={2}>
            <FeatureCard
              title="Theme Aware"
              description="Colors and gradients adapt to the current theme settings."
              icon={Palette}
              href="/docs/theming"
            />
            <FeatureCard
              title="Magic Gradient"
              description="Hover effects use CSS variables for consistent theming."
              icon={Sparkles}
              href="/docs/effects"
            />
          </FeatureCardList>
        </div>
      </div>
    </div>
  ),
};

// Individual size examples
export const SmallSize: Story = {
  args: {
    title: "Compact Feature",
    description: "Perfect for dense layouts and overview pages.",
    icon: Zap,
    size: "sm",
  },
};

export const LargeSize: Story = {
  args: {
    title: "Hero Feature",
    description:
      "Large size for prominent display with generous spacing and enhanced visual impact.",
    icon: Rocket,
    size: "lg",
  },
};
