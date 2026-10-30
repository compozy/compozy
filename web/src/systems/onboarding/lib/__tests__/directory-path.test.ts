import { describe, expect, it } from "vitest";

import {
  completionTarget,
  contractHome,
  expandHome,
  folderName,
  normalizeDirectoryPath,
  suggestionValue,
} from "../directory-path";
import { DIRECTORY_PATH_COPY, judgeDirectoryPath } from "../directory-path-field";

describe("directory path text", () => {
  it("Should expand only `~` and `~/…` against the daemon home", () => {
    expect(expandHome("~", "/Users/ada")).toBe("/Users/ada");
    expect(expandHome("~/code", "/Users/ada/")).toBe("/Users/ada/code");
    expect(expandHome("~bob/code", "/Users/ada")).toBe("~bob/code");
    expect(expandHome("~/code", null)).toBe("~/code");
    expect(contractHome("/Users/ada/code", "/Users/ada")).toBe("~/code");
    expect(contractHome("/Users/adam", "/Users/ada")).toBe("/Users/adam");
  });

  it("Should keep filesystem roots when trimming trailing separators", () => {
    expect(normalizeDirectoryPath("/Users/ada/code/")).toBe("/Users/ada/code");
    expect(normalizeDirectoryPath("/")).toBe("/");
    expect(normalizeDirectoryPath("C:\\")).toBe("C:\\");
    expect(normalizeDirectoryPath("C:\\Users\\ada\\")).toBe("C:\\Users\\ada");
    // On POSIX a backslash is part of the name, so `/tmp/project\` stays a different folder.
    expect(normalizeDirectoryPath("/tmp/project\\")).toBe("/tmp/project\\");
    expect(folderName("/tmp/a\\b")).toBe("a\\b");
  });

  it("Should split typed text into the folder to list and the name to match", () => {
    expect(completionTarget("/Users/ada/co")).toEqual({ dir: "/Users/ada", prefix: "co" });
    expect(completionTarget("/Users/ada/")).toEqual({ dir: "/Users/ada", prefix: "" });
    expect(completionTarget("/c")).toEqual({ dir: "/", prefix: "c" });
    expect(completionTarget("C:\\Users\\a")).toEqual({ dir: "C:\\Users", prefix: "a" });
    expect(completionTarget("code")).toBeNull();
  });

  it("Should keep the operator's spelling when a suggestion is chosen", () => {
    expect(suggestionValue("~/co", "code", false)).toBe("~/code/");
    expect(suggestionValue("/tmp/pro", "projects", false)).toBe("/tmp/projects/");
    expect(suggestionValue("C:\\U", "Users", true)).toBe("C:\\Users\\");
    expect(suggestionValue("~", "code", false)).toBe("~/code/");
    expect(folderName("~/code/app/")).toBe("app");
  });

  it("Should explain a `~` path when the daemon's home could not be read", () => {
    const judge = (homeUnavailable: boolean) =>
      judgeDirectoryPath({
        typed: "~/code",
        expanded: "~/code",
        candidate: "",
        home: null,
        homeUnavailable,
        validation: null,
        isProject: () => false,
      });
    expect(judge(false)).toEqual({ status: "checking", message: null });
    expect(judge(true)).toEqual({
      status: "invalid",
      message: DIRECTORY_PATH_COPY.homeUnavailable,
    });
  });
});
