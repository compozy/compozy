import * as React from "react";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import {
  Combobox,
  ComboboxChip,
  ComboboxChips,
  ComboboxChipsInput,
  ComboboxCollection,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  useComboboxAnchor,
} from "../combobox";

type City = { value: string; label: string };

const cities: City[] = [
  { value: "albuquerque", label: "Albuquerque" },
  { value: "alexandria", label: "Alexandria" },
  { value: "amsterdam", label: "Amsterdam" },
  { value: "berlin", label: "Berlin" },
];

function SingleExample() {
  return (
    <Combobox items={cities}>
      <ComboboxInput placeholder="Search cities" aria-label="city" />
      <ComboboxContent>
        <ComboboxList>
          <ComboboxEmpty>No matches</ComboboxEmpty>
          <ComboboxCollection>
            {(item: City) => (
              <ComboboxItem key={item.value} value={item}>
                {item.label}
              </ComboboxItem>
            )}
          </ComboboxCollection>
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  );
}

function MultiExample({ onChange }: { onChange?: (next: City[]) => void }) {
  const anchor = useComboboxAnchor();
  const [selected, setSelected] = React.useState<City[]>([]);
  return (
    <Combobox
      items={cities}
      multiple
      value={selected}
      onValueChange={(value: City[]) => {
        setSelected(value);
        onChange?.(value);
      }}
    >
      <ComboboxChips ref={anchor}>
        {selected.map(item => (
          <ComboboxChip key={item.value}>{item.label}</ComboboxChip>
        ))}
        <ComboboxChipsInput aria-label="Tags" />
      </ComboboxChips>
      <ComboboxContent anchor={anchor}>
        <ComboboxList>
          <ComboboxCollection>
            {(item: City) => (
              <ComboboxItem key={item.value} value={item}>
                {item.label}
              </ComboboxItem>
            )}
          </ComboboxCollection>
        </ComboboxList>
      </ComboboxContent>
    </Combobox>
  );
}

describe("Combobox", () => {
  it("Should open on input activation and filter the list as inputValue changes", async () => {
    const user = userEvent.setup();
    render(<SingleExample />);
    const input = screen.getByLabelText("city") as HTMLInputElement;
    await user.click(input);
    await waitFor(() => expect(screen.getByText("Berlin")).toBeInTheDocument());
    fireEvent.change(input, { target: { value: "alb" } });
    await waitFor(() => expect(screen.queryByText("Berlin")).not.toBeInTheDocument());
    expect(screen.getByText("Albuquerque")).toBeInTheDocument();
  }, 15_000);

  it("Should show the empty state when no items match", async () => {
    const user = userEvent.setup();
    render(<SingleExample />);
    const input = screen.getByLabelText("city");
    await user.click(input);
    fireEvent.change(input, { target: { value: "zzzz" } });
    await waitFor(() => expect(screen.getByText("No matches")).toBeInTheDocument());
  });

  it("Should accumulate selections in multi-select mode and emit the array on change", async () => {
    const user = userEvent.setup();
    const changes: City[][] = [];
    render(<MultiExample onChange={next => changes.push(next)} />);
    const input = screen.getByLabelText("Tags") as HTMLInputElement;
    await user.click(input);
    await waitFor(() => expect(within(document.body).getByText("Albuquerque")).toBeInTheDocument());
    await user.click(within(document.body).getByText("Albuquerque"));
    await waitFor(() => expect(changes.at(-1)?.map(c => c.value)).toEqual(["albuquerque"]));
    await user.click(input);
    await user.click(within(document.body).getByText("Berlin"));
    await waitFor(() =>
      expect(
        changes
          .at(-1)
          ?.map(c => c.value)
          .sort()
      ).toEqual(["albuquerque", "berlin"])
    );
  });

  it("Should render a chip per selected item in multi-select mode", async () => {
    const user = userEvent.setup();
    render(<MultiExample />);
    const input = screen.getByLabelText("Tags");
    await user.click(input);
    await user.click(within(document.body).getByText("Berlin"));
    await waitFor(() => {
      const chip = document.querySelector("[data-slot=combobox-chip]");
      expect(chip).not.toBeNull();
    });
  });

  it("Should mount the input group and open the bordered popup in single-select mode", async () => {
    const user = userEvent.setup();
    render(<SingleExample />);

    const inputGroup = document.querySelector(
      "[data-slot='combobox-input-group']"
    ) as HTMLElement | null;
    const input = screen.getByLabelText("city");

    expect(inputGroup).not.toBeNull();

    await user.click(input);
    await waitFor(() => expect(screen.getByText("Berlin")).toBeInTheDocument());

    const content = document.body.querySelector(
      "[data-slot='combobox-content']"
    ) as HTMLElement | null;

    expect(content).not.toBeNull();
  });

  it("Should render the input trigger button through the combobox trigger primitive", async () => {
    const user = userEvent.setup();
    render(<SingleExample />);

    const trigger = document.querySelector<HTMLButtonElement>(
      "[data-slot='combobox-input-trigger']"
    );

    expect(trigger).not.toBeNull();
    expect(trigger?.querySelector("svg")).not.toBeNull();

    await user.click(trigger!);
    await waitFor(() => expect(screen.getByText("Berlin")).toBeInTheDocument());
  });

  it("Should expose useComboboxAnchor as a MutableRefObject", () => {
    let anchorRef: ReturnType<typeof useComboboxAnchor> | undefined;
    function Harness() {
      anchorRef = useComboboxAnchor();
      return null;
    }
    render(<Harness />);
    expect(anchorRef).toBeDefined();
    expect(anchorRef?.current).toBeNull();
  });
});
