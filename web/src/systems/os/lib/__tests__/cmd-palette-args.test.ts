// Suite: cmd-palette inline arguments
// Invariant: a command's declared arguments decide what the bar collects and
// what blocks ⏎ — required fields must be filled, typed fields must hold a value
// their type allows, and the first field that fails is the one that takes focus.
// Only a clean pass produces values, and a password value exists nowhere except
// those values (US-015, Safety Invariant 6).
// Owning layer: the pure argument state machine.
// Boundary OUT: field rendering, ⇥ traversal and Esc restore (palette-root
// suite), and what the seam does with the submitted values (dispatch suite).
import { describe, expect, it } from "vitest";

import {
  commandNeedsArguments,
  createArgsState,
  filterArgOptions,
  setArgValue,
  submitArgs,
  validateArgValue,
  type PaletteArgsState,
} from "../cmd-palette-args";
import type { ResolvedPaletteCommand } from "../cmd-palette-types";

/** The `_dx.md` capture fixture: a required text field and an optional dropdown. */
function captureCommand(
  args: ResolvedPaletteCommand["arguments"] = [
    { name: "title", type: "text", required: true, placeholder: "Note title" },
    { name: "tag", type: "dropdown", required: false, options: ["inbox", "idea"] },
  ]
): ResolvedPaletteCommand {
  return {
    id: "ext.notes.capture",
    title: "Capture note",
    section: "Notes",
    icon: "notebook-pen",
    source: "ext.notes",
    bindings: ["alt+shift+KeyN"],
    alias: "cap",
    destructive: false,
    availability_exempt: false,
    arguments: args,
    action: { kind: "tool", tool: "ext__notes__capture" },
    execution: { retry_safe: false, single_flight: true },
    visible: true,
    available: true,
    reason: "",
    chords: ["⌥⇧N"],
  } as ResolvedPaletteCommand;
}

function fill(state: PaletteArgsState, values: Record<string, string>): PaletteArgsState {
  return Object.entries(values).reduce(
    (current, [name, value]) => setArgValue(current, name, value),
    state
  );
}

describe("cmd-palette argument fields (UT-120)", () => {
  it("Should build one field per declared argument in declared order", () => {
    const state = createArgsState(captureCommand());
    expect(commandNeedsArguments(captureCommand())).toBe(true);
    expect(state.fields.map(field => field.name)).toEqual(["title", "tag"]);
    expect(state.fields[0]).toMatchObject({
      type: "text",
      required: true,
      placeholder: "Note title",
      value: "",
    });
    expect(state.fields[1]).toMatchObject({ type: "dropdown", required: false });
    expect(state.fields[1]?.options).toEqual(["inbox", "idea"]);
  });

  it("Should report no arguments for a command that declares none", () => {
    expect(commandNeedsArguments(captureCommand([]))).toBe(false);
  });

  it("Should submit the filled values once every field passes", () => {
    const state = fill(createArgsState(captureCommand()), {
      title: "Standup follow-ups",
      tag: "inbox",
    });
    const submission = submitArgs(state);
    expect(submission.values).toEqual({ title: "Standup follow-ups", tag: "inbox" });
    expect(submission.state.focusField).toBeNull();
  });

  it("Should omit an optional field the operator left empty", () => {
    const state = fill(createArgsState(captureCommand()), { title: "Standup follow-ups" });
    expect(submitArgs(state).values).toEqual({ title: "Standup follow-ups" });
  });

  it("Should degrade an argument type it does not know to text rather than dropping the field", () => {
    const state = createArgsState(
      captureCommand([{ name: "shape", type: "polygon", required: true }])
    );
    expect(state.fields[0]?.type).toBe("text");
  });
});

describe("cmd-palette argument validation (UT-121)", () => {
  it("Should block submit on the first empty required field and focus it", () => {
    const submission = submitArgs(createArgsState(captureCommand()));
    expect(submission.values).toBeNull();
    expect(submission.state.focusField).toBe("title");
    expect(submission.state.fields[0]?.error).toBe("required");
    expect(submission.state.fields[1]?.error).toBe("");
  });

  it("Should reject a pasted value the argument type does not allow", () => {
    const state = fill(createArgsState(captureCommand()), {
      title: "Standup follow-ups",
      tag: "archive",
    });
    expect(state.fields[1]?.error).toBe("expected one of inbox, idea");
    const submission = submitArgs(state);
    expect(submission.values).toBeNull();
    expect(submission.state.focusField).toBe("tag");
  });

  it("Should clear a field message as soon as the value becomes valid", () => {
    const blocked = fill(createArgsState(captureCommand()), { tag: "archive" });
    expect(blocked.fields[1]?.error).not.toBe("");
    const fixed = setArgValue(blocked, "tag", "idea");
    expect(fixed.fields[1]?.error).toBe("");
  });

  it("Should accept only boolean words in a checkbox argument", () => {
    const field = createArgsState(
      captureCommand([{ name: "force", type: "checkbox", required: true }])
    ).fields[0];
    if (field === undefined) throw new Error("checkbox fixture did not build a field");
    expect(validateArgValue(field, "yes")).toBe("");
    expect(validateArgValue(field, "maybe")).toBe("expected true or false");
  });

  it("Should coerce a checkbox value to a boolean for the daemon", () => {
    const state = fill(
      createArgsState(captureCommand([{ name: "force", type: "checkbox", required: true }])),
      { force: "true" }
    );
    expect(submitArgs(state).values).toEqual({ force: true });
  });

  it("Should type-to-filter a dropdown's options", () => {
    const field = createArgsState(captureCommand()).fields[1];
    if (field === undefined) throw new Error("dropdown fixture did not build a field");
    expect(filterArgOptions(field, "in")).toEqual(["inbox"]);
    expect(filterArgOptions(field, "")).toEqual(["inbox", "idea"]);
  });
});

describe("cmd-palette password arguments (US-015.EC-4)", () => {
  const secretCommand = captureCommand([
    { name: "token", type: "password", required: true },
    { name: "title", type: "text", required: false, placeholder: "Note title" },
  ]);

  it("Should keep a password value out of everything except the submitted values", () => {
    const state = fill(createArgsState(secretCommand), { token: "n0tes-sync-tok" });
    const submission = submitArgs(state);
    expect(submission.values).toEqual({ token: "n0tes-sync-tok" });
    // The state that renders and the state that reports carry the value once
    // each, and nothing else in the machine echoes it.
    const rendered = JSON.stringify({
      commandId: submission.state.commandId,
      title: submission.state.title,
      focusField: submission.state.focusField,
      errors: submission.state.fields.map(field => field.error),
      placeholders: submission.state.fields.map(field => field.placeholder),
    });
    expect(rendered).not.toContain("n0tes-sync-tok");
  });

  it("Should still block on a missing required password without echoing a value", () => {
    const submission = submitArgs(createArgsState(secretCommand));
    expect(submission.values).toBeNull();
    expect(submission.state.focusField).toBe("token");
    expect(submission.state.fields[0]?.error).toBe("required");
  });
});
