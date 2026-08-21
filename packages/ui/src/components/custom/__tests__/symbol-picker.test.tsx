import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Briefcase, Compass, Megaphone, Rocket } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import type { SymbolSwatch, SymbolValue } from "../../../lib/symbol-palette";
import type { KindIconRegistry } from "../kind-icon-registry";
import { SymbolPicker } from "../symbol-picker";

const ICON_REGISTRY = {
  megaphone: Megaphone,
  briefcase: Briefcase,
  rocket: Rocket,
  compass: Compass,
} satisfies KindIconRegistry;

const ICONS = [
  { name: "megaphone", keywords: "marketing announce" },
  { name: "briefcase", keywords: "work" },
  { name: "rocket", keywords: "launch" },
  { name: "compass", keywords: "explore" },
];

const EMOJIS = [
  { value: "🚀", label: "rocket", keywords: "launch" },
  { value: "🌱", label: "seedling", keywords: "growth" },
];

const SWATCHES: SymbolSwatch[] = [
  { label: "Gray", value: "#8a8f98" },
  { label: "Violet", value: "#c26ad6" },
];

interface HarnessProps {
  symbol?: SymbolValue;
  color?: string;
  onSymbolChange?: (next: SymbolValue) => void;
  onColorChange?: (next: string) => void;
  onColorValidityChange?: (valid: boolean) => void;
}

function Harness({
  symbol = { kind: "icon", value: "megaphone" },
  color = "#c26ad6",
  onSymbolChange = () => {},
  onColorChange = () => {},
  onColorValidityChange,
}: HarnessProps) {
  return (
    <SymbolPicker
      color={color}
      onColorChange={onColorChange}
      onColorValidityChange={onColorValidityChange}
      symbol={symbol}
      onSymbolChange={onSymbolChange}
      icons={ICONS}
      iconRegistry={ICON_REGISTRY}
      emojis={EMOJIS}
      swatches={SWATCHES}
    />
  );
}

describe("SymbolPicker", () => {
  it("Should mark the chosen icon as the selected option", () => {
    render(<Harness />);
    const selected = screen.getByRole("option", { name: "megaphone" });
    expect(selected).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("option", { name: "briefcase" })).toHaveAttribute(
      "aria-selected",
      "false"
    );
  });

  it("Should emit the picked icon with its kind", async () => {
    const user = userEvent.setup();
    const onSymbolChange = vi.fn();
    render(<Harness onSymbolChange={onSymbolChange} />);
    await user.click(screen.getByRole("option", { name: "rocket" }));
    expect(onSymbolChange).toHaveBeenCalledWith({ kind: "icon", value: "rocket" });
  });

  it("Should filter the grid by search text", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.type(screen.getByLabelText("Search icons"), "launch");
    expect(screen.getByRole("option", { name: "rocket" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "briefcase" })).not.toBeInTheDocument();
  });

  it("Should name the query and the other tab when nothing matches", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.type(screen.getByLabelText("Search icons"), "dragon");
    expect(screen.getByText('No icons match "dragon". Try the Emojis tab.')).toBeInTheDocument();
  });

  it("Should swap to the emoji grid and clear the query", async () => {
    const user = userEvent.setup();
    const onSymbolChange = vi.fn();
    render(<Harness onSymbolChange={onSymbolChange} />);
    await user.type(screen.getByLabelText("Search icons"), "launch");
    await user.click(screen.getByRole("button", { name: "Emojis" }));
    expect(screen.getByLabelText("Search emojis")).toHaveValue("");
    await user.click(screen.getByRole("option", { name: "seedling" }));
    expect(onSymbolChange).toHaveBeenCalledWith({ kind: "emoji", value: "🌱" });
  });

  it("Should move the active option with the arrow keys and commit on Enter", async () => {
    const user = userEvent.setup();
    const onSymbolChange = vi.fn();
    render(<Harness onSymbolChange={onSymbolChange} />);
    const grid = screen.getByRole("listbox", { name: "Icons" });
    grid.focus();
    await user.keyboard("{ArrowRight}{Enter}");
    expect(onSymbolChange).toHaveBeenCalledWith({ kind: "icon", value: "briefcase" });
  });

  it("Should track the active option through aria-activedescendant", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const grid = screen.getByRole("listbox", { name: "Icons" });
    grid.focus();
    await user.keyboard("{End}");
    const last = within(grid).getByRole("option", { name: "compass" });
    expect(grid).toHaveAttribute("aria-activedescendant", last.id);
  });

  it("Should commit a valid custom color and normalize it", async () => {
    const user = userEvent.setup();
    const onColorChange = vi.fn();
    render(<Harness onColorChange={onColorChange} />);
    const field = screen.getByLabelText("Custom color");
    await user.clear(field);
    await user.paste("4EA7FC");
    expect(onColorChange).toHaveBeenLastCalledWith("#4ea7fc");
  });

  it("Should report an invalid color inline and keep the last good one", async () => {
    const user = userEvent.setup();
    const onColorChange = vi.fn();
    render(<Harness onColorChange={onColorChange} />);
    const field = screen.getByLabelText("Custom color");
    await user.clear(field);
    await user.paste("12ZZ");
    expect(field).toHaveAttribute("aria-invalid", "true");
    expect(screen.getByText("Enter a color like #4ea7fc.")).toBeInTheDocument();
    expect(onColorChange).not.toHaveBeenCalled();
    // The identity keeps its previous color — nothing flashed or reset.
    expect(screen.getByRole("option", { name: "Violet" })).toHaveAttribute("aria-selected", "true");
  });

  it("Should report color validity to the containing form", async () => {
    const user = userEvent.setup();
    const onColorValidityChange = vi.fn();
    render(<Harness onColorValidityChange={onColorValidityChange} />);
    const field = screen.getByLabelText("Custom color");

    await user.clear(field);
    await user.paste("12ZZ");
    expect(onColorValidityChange).toHaveBeenLastCalledWith(false);

    await user.click(screen.getByRole("option", { name: "Gray" }));
    expect(onColorValidityChange).toHaveBeenLastCalledWith(true);
  });

  it("Should describe the invalid field by its error message", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const field = screen.getByLabelText("Custom color");
    await user.clear(field);
    await user.paste("nope");
    const describedBy = field.getAttribute("aria-describedby");
    expect(describedBy).not.toBeNull();
    expect(document.getElementById(describedBy!)).toHaveTextContent("Enter a color like #4ea7fc.");
  });

  it("Should select a suggested swatch", async () => {
    const user = userEvent.setup();
    const onColorChange = vi.fn();
    render(<Harness onColorChange={onColorChange} />);
    await user.click(screen.getByRole("option", { name: "Gray" }));
    expect(onColorChange).toHaveBeenCalledWith("#8a8f98");
  });
});
