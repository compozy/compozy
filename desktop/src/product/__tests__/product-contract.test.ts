import { describe, expect, it } from "vitest";

import {
  PRODUCT_METHODS,
  validProductEventPayload,
  validProductParams,
  validProductResponse,
} from "../product-contract";

describe("product preload contract", () => {
  it("Should allow only the declared product methods", () => {
    expect(PRODUCT_METHODS.has("global_shortcuts.sync")).toBe(true);
    expect(PRODUCT_METHODS.has("global_shortcuts.status")).toBe(true);
    expect(PRODUCT_METHODS.has("theme.set")).toBe(true);
    expect(PRODUCT_METHODS.has("dialog.pick_directory")).toBe(true);
    expect(PRODUCT_METHODS.has("files.open_in_editor")).toBe(true);
    expect(PRODUCT_METHODS.has("shell.execute")).toBe(false);
  });

  it("Should validate sync parameters strictly", () => {
    expect(
      validProductParams("global_shortcuts.sync", {
        bindings: [{ command_id: "palette.summon.global", chord: "meta+shift+Space" }],
      })
    ).toBe(true);
    expect(
      validProductParams("global_shortcuts.sync", {
        bindings: [{ command_id: "palette.summon.global", chord: "meta+shift+Space", extra: true }],
      })
    ).toBe(false);
    expect(validProductParams("global_shortcuts.status", { extra: true })).toBe(false);
  });

  it("Should validate registration responses and summon payloads", () => {
    expect(
      validProductResponse("global_shortcuts.status", [
        {
          command_id: "palette.summon.global",
          intended_chord: "meta+shift+Space",
          active_chord: "meta+shift+Space",
          status: "registered",
        },
      ])
    ).toBe(true);
    expect(
      validProductResponse("global_shortcuts.status", [
        {
          command_id: "palette.summon.global",
          intended_chord: "meta+shift+Space",
          status: "unknown",
        },
      ])
    ).toBe(false);
    expect(
      validProductResponse("global_shortcuts.status", [
        {
          command_id: "palette.summon.global",
          intended_chord: "meta+shift+Space",
          status: "registered",
          extra: true,
        },
      ])
    ).toBe(false);
    expect(validProductEventPayload("shell:summon", { command_id: "palette.open" })).toBe(true);
    expect(
      validProductEventPayload("shell:summon", { command_id: "palette.open", extra: true })
    ).toBe(false);
  });

  it("Should accept only a known theme preference and echo it back", () => {
    expect(validProductParams("theme.set", { preference: "system" })).toBe(true);
    expect(validProductParams("theme.set", { preference: "sepia" })).toBe(false);
    expect(validProductParams("theme.set", { preference: "dark", extra: true })).toBe(false);
    expect(validProductResponse("theme.set", { preference: "light" })).toBe(true);
    expect(validProductResponse("theme.set", [])).toBe(false);
  });

  it("Should accept only an absolute folder to open in and an absolute or canceled pick", () => {
    expect(validProductParams("dialog.pick_directory", {})).toBe(true);
    expect(validProductParams("dialog.pick_directory", { default_path: "/Users/ada" })).toBe(true);
    expect(validProductParams("dialog.pick_directory", { default_path: "C:\\Users\\ada" })).toBe(
      true
    );
    expect(validProductParams("dialog.pick_directory", { default_path: "~/code" })).toBe(false);
    expect(validProductParams("dialog.pick_directory", { default_path: "\\\\server\\share" })).toBe(
      true
    );
    expect(validProductParams("dialog.pick_directory", { default_path: "\\\\server" })).toBe(false);
    expect(validProductParams("dialog.pick_directory", { default_path: "/tmp\u0000x" })).toBe(
      false
    );
    expect(validProductParams("dialog.pick_directory", { default_path: "/x", extra: true })).toBe(
      false
    );
    expect(validProductResponse("dialog.pick_directory", { path: "/Users/ada/code" })).toBe(true);
    expect(validProductResponse("dialog.pick_directory", { path: null })).toBe(true);
    expect(validProductResponse("dialog.pick_directory", { path: "relative/dir" })).toBe(false);
    expect(validProductResponse("dialog.pick_directory", { path: "/x", canceled: false })).toBe(
      false
    );
    expect(validProductResponse("dialog.pick_directory", {})).toBe(false);
  });

  it("Should accept only a local file target, bounded positions, and local absolute roots", () => {
    const open = (params: unknown) => validProductParams("files.open_in_editor", params);
    expect(open({ path: "/repo/a.ts", line: 3, column: 7, roots: ["/repo"] })).toBe(true);
    expect(open({ path: "~/x", roots: [] })).toBe(true);
    expect(open({ path: "C:\\repo\\a.ts", roots: ["C:/repo"] })).toBe(true);
    expect(open({ path: "relative/a.ts", roots: [] })).toBe(false);
    expect(open({ path: "/", roots: [] })).toBe(false);
    expect(open({ path: "~", roots: [] })).toBe(false);
    expect(open({ path: "/a\u0000b", roots: [] })).toBe(false);
    expect(open({ path: `/${"a".repeat(1024)}`, roots: [] })).toBe(false);
    expect(open({ path: "/a.ts", line: 0, roots: [] })).toBe(false);
    expect(open({ path: "/a.ts", column: 1.5, roots: [] })).toBe(false);
    expect(open({ path: "/a.ts", line: 1_000_000_000, roots: [] })).toBe(false);
    expect(open({ path: "/a.ts", roots: Array.from({ length: 9 }, (_, i) => `/r${i}`) })).toBe(
      false
    );
    expect(open({ path: "/a.ts", roots: Array.from({ length: 8 }, (_, i) => `/r${i}`) })).toBe(
      true
    );
    expect(open({ path: "/a.ts", roots: ["repo"] })).toBe(false);
    expect(open({ path: "/a.ts", roots: ["~/repo"] })).toBe(false);
    expect(open({ path: "/a.ts", roots: [], command: "sh" })).toBe(false);
    expect(open({ path: "/a.ts" })).toBe(false);
    for (const network of [
      "\\\\host\\share\\x.txt",
      "\\\\?\\C:\\x",
      "\\\\.\\pipe\\x",
      "//host/x",
    ]) {
      expect(open({ path: network, roots: [] })).toBe(false);
      expect(open({ path: "/a.ts", roots: [network] })).toBe(false);
    }
    expect(open({ path: "~//host/x", roots: [] })).toBe(false);
  });

  it("Should accept only the three open results and the six error codes", () => {
    const result = (value: unknown) => validProductResponse("files.open_in_editor", value);
    expect(result({ status: "opened" })).toBe(true);
    expect(result({ status: "cancelled" })).toBe(true);
    for (const code of [
      "not_found",
      "not_a_file",
      "editor_invalid",
      "editor_unavailable",
      "editor_not_configured",
      "editor_failed",
    ]) {
      expect(result({ status: "error", code, message: "x" })).toBe(true);
    }
    expect(result({ status: "error", code: "spawned", message: "x" })).toBe(false);
    expect(result({ status: "error", code: "not_found" })).toBe(false);
    expect(result({ status: "opened", pid: 1 })).toBe(false);
    expect(result({ status: "launched" })).toBe(false);
    expect(result(null)).toBe(false);
  });
});
