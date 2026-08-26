// Suite: custom skill-source editor.
// Invariant: a rejected entry is refused next to the input with the daemon's own
// code, and an entry with no measurement yet never renders a count.
// Owning layer: settings domain component.
// Canonical suite: this file (the editor has no other owner).
// Boundary IN: draft entries + the validator. Boundary OUT: HTTP and the draft store.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { settingsSkillSourcesFixture } from "@/systems/settings/mocks";
import type { SettingsSkillSource } from "@/systems/settings";

import { validateSkillSourceEntry } from "../../lib/skill-source-draft";
import { groupSkillSources } from "../../lib/skill-sources-view";
import { SettingsSkillCustomSources } from "../settings-skill-custom-sources";

const sources = settingsSkillSourcesFixture as readonly SettingsSkillSource[];
const customViews = groupSkillSources(sources).custom;

function renderEditor(
  overrides: {
    entries?: string[];
    workspaceScope?: boolean;
    onAdd?: (path: string) => void;
    onRemove?: (path: string) => void;
  } = {}
) {
  const entries = overrides.entries ?? ["~/team-skills"];
  const onAdd = overrides.onAdd ?? vi.fn();
  const onRemove = overrides.onRemove ?? vi.fn();
  render(
    <SettingsSkillCustomSources
      disabled={false}
      entries={entries}
      onAdd={onAdd}
      onRemove={onRemove}
      sources={customViews}
      validate={entry =>
        validateSkillSourceEntry(entry, {
          customEntries: entries,
          sources,
          workspaceScope: overrides.workspaceScope ?? false,
        })
      }
    />
  );
  return { onAdd, onRemove };
}

describe("SettingsSkillCustomSources", () => {
  it("Should add a folder the daemon has no objection to", async () => {
    const user = userEvent.setup();
    const { onAdd } = renderEditor();

    await user.type(
      screen.getByTestId("settings-page-skills-custom-sources-input"),
      "~/client-acme/skills"
    );
    await user.click(screen.getByTestId("settings-page-skills-custom-sources-add"));

    expect(onAdd).toHaveBeenCalledWith("~/client-acme/skills");
    expect(
      screen.queryByTestId("settings-page-skills-custom-sources-error")
    ).not.toBeInTheDocument();
  });

  it("Should refuse a folder already on the list and name the source that owns it", async () => {
    const user = userEvent.setup();
    const { onAdd } = renderEditor();

    await user.type(
      screen.getByTestId("settings-page-skills-custom-sources-input"),
      "/Users/ana/team-skills"
    );
    await user.click(screen.getByTestId("settings-page-skills-custom-sources-add"));

    const error = screen.getByTestId("settings-page-skills-custom-sources-error");
    expect(error).toHaveTextContent("This folder is already on the list as team-skills.");
    expect(error).toHaveTextContent("duplicate_skill_source");
    expect(onAdd).not.toHaveBeenCalled();
    expect(screen.getByTestId("settings-page-skills-custom-sources-input")).toHaveValue(
      "/Users/ana/team-skills"
    );
  });

  it("Should refuse a preset's configured home-relative folder", async () => {
    const user = userEvent.setup();
    const { onAdd } = renderEditor();

    await user.type(
      screen.getByTestId("settings-page-skills-custom-sources-input"),
      "~/.agents/skills"
    );
    await user.click(screen.getByTestId("settings-page-skills-custom-sources-add"));

    expect(screen.getByTestId("settings-page-skills-custom-sources-error")).toHaveTextContent(
      "already on the list as Universal (.agents)"
    );
    expect(onAdd).not.toHaveBeenCalled();
  });

  it("Should refuse a project-relative folder outside workspace scope", async () => {
    const user = userEvent.setup();
    const { onAdd } = renderEditor();

    await user.type(
      screen.getByTestId("settings-page-skills-custom-sources-input"),
      "./vendor/skills"
    );
    await user.click(screen.getByTestId("settings-page-skills-custom-sources-add"));

    const error = screen.getByTestId("settings-page-skills-custom-sources-error");
    expect(error).toHaveTextContent("can only be added in Workspace scope");
    expect(error).toHaveTextContent("invalid_source_path");
    expect(onAdd).not.toHaveBeenCalled();
  });

  it("Should accept a project-relative folder at workspace scope", async () => {
    const user = userEvent.setup();
    const { onAdd } = renderEditor({ workspaceScope: true });

    await user.type(
      screen.getByTestId("settings-page-skills-custom-sources-input"),
      "./vendor/skills"
    );
    await user.click(screen.getByTestId("settings-page-skills-custom-sources-add"));

    expect(onAdd).toHaveBeenCalledWith("./vendor/skills");
  });

  it("Should remove a measured folder by its configured path", async () => {
    const user = userEvent.setup();
    const { onRemove } = renderEditor();

    await user.click(screen.getByTestId("settings-page-skills-source-team-skills-remove"));

    expect(onRemove).toHaveBeenCalledWith("~/team-skills");
  });

  it("Should show an unsaved folder without inventing a count for it", () => {
    renderEditor({ entries: ["~/team-skills", "~/client-acme/skills"] });

    const pending = screen.getByTestId(
      "settings-page-skills-custom-sources-pending-~/client-acme/skills"
    );
    expect(pending).toHaveTextContent("not scanned yet");
    expect(pending).not.toHaveTextContent("0 skills");
  });
});
