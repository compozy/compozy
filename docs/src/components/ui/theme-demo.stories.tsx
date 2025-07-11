import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Check, Moon, Palette, Sun } from "lucide-react";
import { useTheme } from "next-themes";

// Demo component to show theme switching
const ThemeDemo = () => {
  const { theme, setTheme } = useTheme();

  return (
    <div className="p-8 space-y-6">
      <div className="text-foreground">
        <h1 className="text-3xl font-bold mb-2">Fumadocs Theme Integration</h1>
        <p className="text-muted-foreground mb-6">
          Current theme: <span className="font-semibold text-primary">{theme}</span>
        </p>

        <div className="space-y-6">
          {/* Theme Toggle */}
          <div className="flex items-center gap-4">
            <button
              onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
              className="flex items-center gap-2 px-4 py-2 bg-secondary text-secondary-foreground rounded-md hover:bg-secondary/80 transition-colors"
            >
              {theme === "dark" ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
              Toggle Theme
            </button>
          </div>

          {/* Color Showcase */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="p-4 bg-card rounded-lg border border-border">
              <h2 className="text-lg font-semibold mb-2">Card Component</h2>
              <p className="text-muted-foreground">
                This card uses the fumadocs theme colors with proper contrast.
              </p>
            </div>

            <div className="p-4 bg-primary text-primary-foreground rounded-lg">
              <h2 className="text-lg font-semibold mb-2">Primary Color</h2>
              <p>This showcases the primary color scheme from fumadocs.</p>
            </div>

            <div className="p-4 bg-secondary text-secondary-foreground rounded-lg">
              <h2 className="text-lg font-semibold mb-2">Secondary Color</h2>
              <p>Secondary color palette for subtle UI elements.</p>
            </div>

            <div className="p-4 bg-accent text-accent-foreground rounded-lg">
              <h2 className="text-lg font-semibold mb-2">Accent Color</h2>
              <p>Accent colors for highlighting important elements.</p>
            </div>

            <div className="p-4 bg-muted text-muted-foreground rounded-lg">
              <h2 className="text-lg font-semibold mb-2">Muted Color</h2>
              <p>Muted colors for less prominent UI elements.</p>
            </div>

            <div className="p-4 bg-destructive text-destructive-foreground rounded-lg">
              <h2 className="text-lg font-semibold mb-2">Destructive Color</h2>
              <p>Used for destructive actions and error states.</p>
            </div>
          </div>

          {/* Theme Verification */}
          <div className="p-4 bg-popover rounded-lg border border-border">
            <h3 className="font-semibold mb-3 flex items-center gap-2">
              <Palette className="w-5 h-5" />
              Theme Integration Status
            </h3>
            <ul className="space-y-2 text-sm">
              <li className="flex items-center gap-2">
                <Check className="w-4 h-4 text-green-600" />
                <span>Fumadocs theme colors applied</span>
              </li>
              <li className="flex items-center gap-2">
                <Check className="w-4 h-4 text-green-600" />
                <span>Dark/Light mode switching enabled</span>
              </li>
              <li className="flex items-center gap-2">
                <Check className="w-4 h-4 text-green-600" />
                <span>CSS variables synchronized</span>
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
};

const meta = {
  title: "Theme/Demo",
  component: ThemeDemo,
  parameters: {
    layout: "fullscreen",
  },
  tags: ["autodocs"],
} satisfies Meta<typeof ThemeDemo>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

// Force dark theme for this story
export const ForcedDark: Story = {
  parameters: {
    themes: {
      themeOverride: "dark",
    },
  },
};

// Force light theme for this story
export const ForcedLight: Story = {
  parameters: {
    themes: {
      themeOverride: "light",
    },
  },
};
