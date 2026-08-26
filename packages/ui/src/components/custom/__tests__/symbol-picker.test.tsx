import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";

import { identityColorsFor } from "../../../lib/identity-palette";
import {
  SYMBOL_PICKER_DEFAULT_LABELS,
  type SymbolKind,
  type SymbolPickerLabels,
  type SymbolSwatch,
  type SymbolValue,
} from "../../../lib/symbol-palette";
import { SymbolPicker } from "../symbol-picker";

const SPRITE_URL = "/assets/lucide-sprite.svg";
const EMOJIBASE_URL = "/vendor/emojibase";

const ICONS = [
  { name: "megaphone", keywords: "marketing announce" },
  { name: "briefcase", keywords: "work" },
  { name: "rocket", keywords: "launch" },
  { name: "compass", keywords: "explore" },
];

const SWATCHES: SymbolSwatch[] = [
  { label: "Gray", value: "#8a8f98" },
  { label: "Violet", value: "#c26ad6" },
];

// Minimal Emojibase payloads in the exact shape frimousse fetches.
const EMOJI_DATA = [
  {
    label: "rocket",
    hexcode: "1F680",
    tags: ["launch", "space"],
    emoji: "🚀",
    text: "",
    type: 1,
    order: 1,
    group: 5,
    subgroup: 56,
    version: 0.6,
  },
  {
    label: "seedling",
    hexcode: "1F331",
    tags: ["plant", "growth"],
    emoji: "🌱",
    text: "",
    type: 1,
    order: 2,
    group: 3,
    subgroup: 41,
    version: 0.6,
  },
];
const EMOJI_MESSAGES = {
  groups: [
    { key: "animals-nature", message: "animals & nature", order: 3 },
    { key: "travel-places", message: "travel & places", order: 5 },
  ],
  skinTones: [
    { key: "dark", message: "dark skin tone" },
    { key: "light", message: "light skin tone" },
  ],
  subgroups: [],
};

beforeAll(() => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const body = url.endsWith("/data.json")
        ? EMOJI_DATA
        : url.endsWith("/messages.json")
          ? EMOJI_MESSAGES
          : null;
      if (body === null) throw new Error(`unexpected fetch ${url}`);
      if (init?.method === "HEAD") return new Response(null, { status: 200 });
      return Response.json(body);
    })
  );
});

afterAll(() => {
  vi.unstubAllGlobals();
});

interface HarnessProps {
  symbol?: SymbolValue;
  color?: string;
  surface?: string;
  labels?: SymbolPickerLabels;
  swatches?: readonly SymbolSwatch[];
  iconsLoading?: boolean;
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
  iconsLoading = false,
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
      iconsLoading={iconsLoading}
      spriteUrl={SPRITE_URL}
      emojibaseUrl={EMOJIBASE_URL}
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

  // Invariant: every icon cell renders out of the shared sprite, so any catalog
  // slug the daemon accepts is renderable without a per-icon import.
  // Owning layer: SymbolPicker's sprite rendering contract.
  // Canonical suite: this SymbolPicker component interaction suite.
  it("Should render icon cells through the sprite url", () => {
    render(<Harness />);
    const selected = screen.getByRole("option", { name: "megaphone" });
    expect(selected.querySelector("use")).toHaveAttribute("href", `${SPRITE_URL}#megaphone`);
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

  // Invariant: a catalog that has not finished loading announces itself instead
  // of claiming there are no icons.
  // Owning layer: SymbolPicker's async catalog contract.
  // Canonical suite: this SymbolPicker component interaction suite.
  it("Should show a loading state while the catalog loads", () => {
    render(<Harness iconsLoading />);
    expect(screen.getByRole("status", { name: "Loading icons…" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "megaphone" })).not.toBeInTheDocument();
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
  // Owning layer: SymbolPickerIconGrid's color-derivation boundary.
  // Canonical suite: this SymbolPicker component interaction suite.
  it("Should render when the supplied surface is not a hex color", () => {
    render(<Harness surface="var(--color-canvas-soft)" />);
    expect(screen.getByRole("option", { name: "megaphone" })).toBeInTheDocument();
  });

  // Invariant: selected icon ink is measured against the selected identity plate, not the bare panel.
  // Owning layer: SymbolPickerIconGrid's rendered color contract.
  // Canonical suite: this SymbolPicker component interaction suite.
  it("Should use plate-contrast ink for the selected icon", () => {
    const color = "#81597a";
    const surface = "#2a2927";
    render(<Harness color={color} surface={surface} />);
    const selected = screen.getByRole("option", { name: "megaphone" });
    const expected = identityColorsFor(color, surface);
    expect(selected).toHaveStyle({ backgroundColor: expected.bg, color: expected.fg });
  });

  it("Should swap to the emoji pane with its own search and skin tone control", async () => {
    const user = userEvent.setup();
    render(<Harness />);
    await user.type(screen.getByLabelText("Search icons"), "launch");
    await user.click(screen.getByRole("button", { name: "Emojis" }));
    expect(await screen.findByLabelText("Search emojis")).toHaveValue("");
    expect(screen.getByRole("button", { name: "Change skin tone" })).toBeInTheDocument();
  });

  it("Should surface the emoji empty state with the shared no-results copy", async () => {
    const user = userEvent.setup();
    render(<Harness symbol={{ kind: "emoji", value: "🌱" }} />);
    const search = await screen.findByLabelText("Search emojis");
    await user.type(search, "dragon");
    expect(
      await screen.findByText('No emojis match "dragon". Try the Icons tab.')
    ).toBeInTheDocument();
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
  // Owning layer: useSwatchPalette keyboard model as consumed by SymbolPickerColorSection.
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

  // Invariant: the free color area lives in a popover behind the spectrum
  // button — it never grows the host surface — and feeds normalized hex back
  // through onColorChange.
  // Owning layer: SymbolPickerColorSection's custom-area contract.
  // Canonical suite: this SymbolPicker component interaction suite.
  it("Should open the custom color area in a popover from the spectrum button", async () => {
    const user = userEvent.setup();
    render(<Harness color="#8a8f98" />);
    const toggle = screen.getByRole("button", { name: "Pick a custom color" });
    expect(document.querySelector('[data-slot="color-picker"]')).not.toBeInTheDocument();

    await user.click(toggle);
    expect(await screen.findByRole("slider", { name: "Hue" })).toBeInTheDocument();
    expect(document.querySelector('[data-slot="color-picker"]')).toBeInTheDocument();
  });

  it("Should mark the spectrum button while a non-suggested color is active", () => {
    render(<Harness color="#123456" />);
    expect(screen.getByRole("button", { name: "Pick a custom color" })).toHaveAttribute(
      "data-custom-color",
      "true"
    );
  });
});
