import { useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const loopsState = vi.hoisted(() => ({
  current: {
    loops: [] as unknown[],
    error: null as Error | null,
    fetchNextPage: vi.fn(),
    hasNextPage: false,
    isError: false,
    isFetchingNextPage: false,
    isLoading: false,
  },
}));

vi.mock("../../hooks/use-loops", async () => {
  const { loopCatalogFixtures } = await import("../../mocks/fixtures");
  loopsState.current = {
    loops: loopCatalogFixtures,
    error: null,
    fetchNextPage: vi.fn(),
    hasNextPage: false,
    isError: false,
    isFetchingNextPage: false,
    isLoading: false,
  };
  return { useLoops: () => loopsState.current };
});

const { LoopTargetFields } = await import("../target/loop-target-fields");
const { useLoopTargetCatalog } = await import("../../hooks/use-loop-target-catalog");
type LoopTargetDraft = import("../../lib/loop-target").LoopTargetDraft;

function Harness({
  initialValue,
  requiredStartKind = "schedule",
  showMapping,
}: {
  initialValue?: LoopTargetDraft;
  requiredStartKind?: "schedule" | "trigger" | "webhook";
  showMapping?: boolean;
}) {
  const [value, setValue] = useState<LoopTargetDraft>(
    initialValue ?? {
      loop_name: "",
      inputs: {},
      input_mapping: {},
    }
  );
  const catalog = useLoopTargetCatalog("ws", value.loop_name, requiredStartKind);
  return (
    <>
      <LoopTargetFields
        catalog={catalog}
        mode={initialValue ? "edit" : "create"}
        value={value}
        onChange={setValue}
        showMapping={showMapping}
      />
      <output data-testid="loop-target-value">{JSON.stringify(value)}</output>
    </>
  );
}

describe("LoopTargetFields", () => {
  beforeEach(async () => {
    const { loopCatalogFixtures } = await import("../../mocks/fixtures");
    loopsState.current = {
      loops: loopCatalogFixtures,
      error: null,
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isError: false,
      isFetchingNextPage: false,
      isLoading: false,
    };
  });

  function selectLoop(name: string) {
    fireEvent.click(screen.getByTestId("loop-target-select"));
    fireEvent.click(screen.getByText(name, { selector: "[cmdk-item] *" }));
  }

  it("Should list selectable loops and auto-generate a typed input form for the chosen loop", () => {
    render(<Harness />);
    const select = screen.getByTestId("loop-target-select");
    expect(select.tagName).toBe("BUTTON");
    // No inputs render until a loop is picked.
    expect(screen.queryByTestId("loop-target-inputs")).not.toBeInTheDocument();

    selectLoop("implement-tasks");
    const controls = screen.getAllByTestId("loop-input-control").map(node => node.dataset.input);
    expect(controls).toEqual(["slug", "implementer", "auto_commit"]);
    expect(screen.getByTestId("loop-input-field-slug")).toBeInTheDocument();
    expect(screen.getByTestId("loop-input-field-implementer")).toBeInTheDocument();
    expect(screen.getByTestId("loop-input-switch-auto_commit")).toBeInTheDocument();
  });

  it("Should write static input values onto the loop target", () => {
    render(<Harness />);
    selectLoop("implement-tasks");
    fireEvent.change(screen.getByTestId("loop-input-field-slug"), {
      target: { value: "billing-webhooks" },
    });
    expect((screen.getByTestId("loop-input-field-slug") as HTMLInputElement).value).toBe(
      "billing-webhooks"
    );
    expect(JSON.parse(screen.getByTestId("loop-target-value").textContent ?? "{}")).toMatchObject({
      loop_name: "implement-tasks",
      inputs: { slug: "billing-webhooks" },
    });
  });

  it("Should round-trip bounded Live participation and expose only Loop-valid strategies", () => {
    render(
      <Harness
        initialValue={{
          loop_name: "implement-tasks",
          inputs: {},
          input_mapping: {},
          network_participation: {
            mode: "live",
            channel_strategy: "named",
            channel_id: "release-room",
            bounds: { max_wakes: 4 },
          },
        }}
      />
    );

    expect(screen.getByTestId("loop-target-participation-mode")).toHaveValue("live");
    const strategy = screen.getByTestId("loop-target-participation-strategy");
    expect(strategy).toHaveTextContent("Named channel");
    expect(strategy).toHaveTextContent("This Loop run");
    expect(Array.from((strategy as HTMLSelectElement).options, option => option.value)).toEqual([
      "named",
      "loop_run",
    ]);

    fireEvent.change(screen.getByTestId("loop-target-participation-channel"), {
      target: { value: "launch-room" },
    });
    const value = screen.getByTestId("loop-target-value");
    expect(value).toHaveTextContent('"channel_id":"launch-room"');
    expect(value).toHaveTextContent('"max_wakes":4');
  });

  it("Should render the event-payload mapping table only when enabled", () => {
    const { rerender } = render(<Harness />);
    selectLoop("implement-tasks");
    expect(screen.queryByTestId("loop-input-mapping")).not.toBeInTheDocument();

    rerender(<Harness showMapping />);
    expect(screen.getByTestId("loop-input-mapping")).toBeInTheDocument();
    expect(screen.getByTestId("loop-mapping-field-slug")).toBeInTheDocument();
  });

  it("Should render a load error instead of the empty workspace copy", () => {
    loopsState.current = {
      loops: [],
      error: new Error("catalog unavailable"),
      fetchNextPage: vi.fn(),
      hasNextPage: false,
      isError: true,
      isFetchingNextPage: false,
      isLoading: false,
    };
    render(<Harness />);
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load Loops");
    expect(screen.queryByText("No Loops are available in this workspace.")).not.toBeInTheDocument();
  });

  it("Should filter by the authoritative start contract and preserve an incompatible edit target", () => {
    render(
      <Harness
        initialValue={{ loop_name: "review-and-fix", inputs: {}, input_mapping: {} }}
        requiredStartKind="schedule"
      />
    );

    const select = screen.getByTestId("loop-target-select");
    expect(select).toHaveTextContent("review-and-fix");
    expect(select).toHaveTextContent("Not available");
    fireEvent.click(select);
    expect(screen.getByText("implement-tasks", { selector: "[cmdk-item] *" })).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "review-and-fix does not declare the schedule start kind"
    );
  });
});
