import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Briefcase, Compass, Megaphone, Rocket } from "lucide-react";
import { describe, expect, it, vi } from "vitest";

import { identityColorsFor } from "../../../lib/identity-palette";
import {
  SYMBOL_PICKER_DEFAULT_LABELS,
  type SymbolKind,
  type SymbolPickerLabels,
  type SymbolSwatch,
  type SymbolValue,
} from "../../../lib/symbol-palette";
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
  surface?: string;
  labels?: SymbolPickerLabels;
  swatches?: readonly SymbolSwatch[];
  onSymbolChange?: (next: SymbolValue) => void;
  onColorChange?: (next: string) => void;
  onColorValidityChange?: (valid: boolean) => void;
}

function Harness({
  symbol = { kind: "icon", value: "megaphone" },
  color = "#c26ad6",
  surface,
  labels,
  swatches = SWATCHES,
  onSymbolChange = () => {},
  onColorChange = () => {},
  onColorValidityChange,
}: HarnessProps) {
  return (
    <SymbolPicker
      color={color}
      surface={surface}
      labels={labels}
      onColorChange={onColorChange}
      onColorValidityChange={onColorValidityChange}
      symbol={symbol}
      onSymbolChange={onSymbolChange}
      icons={ICONS}
      iconRegistry={ICON_REGISTRY}
      emojis={EMOJIS}
      swatches={swatches}
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

  // Invariant: every visible picker string, including the no-results state, comes from labels.
  // Owning layer: SymbolPicker composition and its public localization contract.
  // Canonical suite: this SymbolPicker component interaction suite.
  it("Should use localized labels for the no-results state", async () => {
    const user = userEvent.setup();
    const labels = {
      ...SYMBOL_PICKER_DEFAULT_LABELS,
      icons: "Ícones",
      emojis: "Emojis",
      searchIcons: "Pesquisar ícones",
      searchEmojis: "Pesquisar emojis",
      noResults: (kind: SymbolKind, query: string, otherTab: string) =>
        `Nenhum ${kind === "icon" ? "ícone" : "emoji"} para "${query}". Tente ${otherTab}.`,
    };
    render(<Harness labels={labels} />);
    await user.type(screen.getByLabelText("Pesquisar ícones"), "dragon");
    expect(screen.getByText('Nenhum ícone para "dragon". Tente Emojis.')).toBeInTheDocument();
  });

  // Invariant: malformed surfaces use the same neutral fallback as identity-palette math.
  // Owning layer: SymbolPickerGrid's color-derivation boundary.
  // Canonical suite: this SymbolPicker component interaction suite.
  it("Should render when the supplied surface is not a hex color", () => {
    render(<Harness surface="var(--color-canvas-soft)" />);
    expect(screen.getByRole("option", { name: "megaphone" })).toBeInTheDocument();
  });

  // Invariant: selected icon ink is measured against the selected identity plate, not the bare panel.
  // Owning layer: SymbolPickerGrid's rendered color contract.
  // Canonical suite: this SymbolPicker component interaction suite.
  it("Should use plate-contrast ink for the selected icon", () => {
    const color = "#81597a";
    const surface = "#2a2927";
    render(<Harness color={color} surface={surface} />);
    const selected = screen.getByRole("option", { name: "megaphone" });
    const expected = identityColorsFor(color, surface);
    expect(selected).toHaveStyle({ backgroundColor: expected.bg, color: expected.fg });
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

  it("Should walk the suggested palette with the arrow keys from one tab stop", async () => {
    const user = userEvent.setup();
    const onColorChange = vi.fn();
    render(<Harness onColorChange={onColorChange} />);
    const palette = screen.getByRole("listbox", { name: "Suggested colors" });

    palette.focus();
    expect(palette).toHaveFocus();
    await user.keyboard("{Home}");
    expect(palette).toHaveAttribute(
      "aria-activedescendant",
      within(palette).getByRole("option", { name: "Gray" }).id
    );

    await user.keyboard("{Enter}");
    expect(onColorChange).toHaveBeenCalledWith("#8a8f98");

    // The palette is one stop, so the next Tab reaches the hex field rather than
    // the second swatch.
    await user.tab();
    expect(screen.getByLabelText("Custom color")).toHaveFocus();
  });

  // Invariant: an empty listbox does not claim to handle navigation or create an invalid cursor.
  // Owning layer: useSwatchPalette keyboard model as consumed by SymbolPickerColorRow.
  // Canonical suite: this SymbolPicker component interaction suite.
  it("Should leave an empty suggested palette idle on navigation keys", async () => {
    const user = userEvent.setup();
    const onColorChange = vi.fn();
    render(<Harness swatches={[]} onColorChange={onColorChange} />);
    const palette = screen.getByRole("listbox", { name: "Suggested colors" });
    palette.focus();

    const event = new KeyboardEvent("keydown", { key: "End", bubbles: true, cancelable: true });
    fireEvent(palette, event);
    expect(event.defaultPrevented).toBe(false);
    await user.keyboard("{Enter}");
    expect(onColorChange).not.toHaveBeenCalled();
    expect(palette).not.toHaveAttribute("aria-activedescendant");
  });
});
