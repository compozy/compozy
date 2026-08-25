// Suite: Settings > Skills sources section rendering.
// Invariant: every row, count, and state word comes from the daemon `sources[]`
// read model — absent, unreadable, and unmeasured never render as zero.
// Owning layer: settings domain component.
// Canonical suite: this file (the section has no other owner).
// Boundary IN: the section's view model. Boundary OUT: HTTP and the draft store.
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { settingsSkillSourcesFixture } from "@/systems/settings/mocks";
import type { SettingsSkillSource } from "@/systems/settings";

import type { SettingsSkillSourcesModel } from "../../hooks/use-settings-skill-sources";
import { groupSkillSources } from "../../lib/skill-sources-view";
import { SettingsSkillSourcesSection } from "../settings-skill-sources-section";

function model(overrides: Partial<SettingsSkillSourcesModel> = {}): SettingsSkillSourcesModel {
  const sources = overrides.groups
    ? []
    : (settingsSkillSourcesFixture as readonly SettingsSkillSource[]);
  return {
    groups: groupSkillSources(sources),
    enabledPresets: ["agents"],
    customEntries: ["~/team-skills"],
    readOnly: false,
    readOnlyReason: null,
    postures: null,
    isDirty: false,
    isSaving: false,
    saveError: null,
    saveErrorCode: null,
    lastLabel: null,
    togglePreset: vi.fn(),
    addCustom: vi.fn(),
    removeCustom: vi.fn(),
    validateEntry: () => null,
    customize: vi.fn(),
    useInherited: vi.fn(),
    inheritPendingKey: null,
    inheritError: null,
    save: vi.fn(),
    reset: vi.fn(),
    ...overrides,
  };
}

function withSources(mutate: (sources: SettingsSkillSource[]) => void): SettingsSkillSourcesModel {
  const sources = structuredClone(settingsSkillSourcesFixture) as SettingsSkillSource[];
  mutate(sources);
  return model({ groups: groupSkillSources(sources) });
}

describe("SettingsSkillSourcesSection", () => {
  it("Should render the always-on Compozy row without a switch", () => {
    render(<SettingsSkillSourcesSection model={model()} />);

    const row = screen.getByTestId("settings-page-skills-source-compozy");
    expect(
      within(row).getByTestId("settings-page-skills-source-compozy-always-on")
    ).toHaveTextContent("always on");
    expect(
      within(row).queryByTestId("settings-page-skills-source-compozy-toggle")
    ).not.toBeInTheDocument();
  });

  it("Should total only the counts the daemon reported", () => {
    render(<SettingsSkillSourcesSection model={model()} />);

    expect(screen.getByTestId("settings-page-skills-source-compozy-count")).toHaveTextContent(
      "12 skills"
    );
    expect(screen.getByTestId("settings-page-skills-source-agents-count")).toHaveTextContent(
      "5 skills"
    );
  });

  it("Should state an absent folder instead of a measured zero", async () => {
    const user = userEvent.setup();
    render(<SettingsSkillSourcesSection model={model()} />);

    await user.click(screen.getByTestId("settings-page-skills-source-claude-disclosure"));
    const state = screen.getByTestId(
      "settings-page-skills-source-claude-root-r_w_claude_3ad1-state"
    );
    expect(state).toHaveTextContent("no folder yet");
    expect(
      screen.queryByTestId("settings-page-skills-source-claude-root-r_w_claude_3ad1-count")
    ).not.toBeInTheDocument();
  });

  it("Should omit every count for an unreadable folder", async () => {
    const user = userEvent.setup();
    const unreadable = withSources(sources => {
      const root = sources[3].roots[0];
      root.readable = false;
    });
    render(<SettingsSkillSourcesSection model={unreadable} />);

    expect(screen.getByTestId("settings-page-skills-source-team-skills-unreadable")).toBeVisible();
    expect(
      screen.queryByTestId("settings-page-skills-source-team-skills-count")
    ).not.toBeInTheDocument();

    await user.click(screen.getByTestId("settings-page-skills-source-team-skills-disclosure"));
    expect(
      screen.getByTestId("settings-page-skills-source-team-skills-root-r_u_custom_a41c-state")
    ).toHaveTextContent("can't read this folder");
  });

  it("Should keep switches operable while the runtime reported no counts", () => {
    const unmeasured = model({
      groups: groupSkillSources(settingsSkillSourcesFixture, false),
    });
    render(<SettingsSkillSourcesSection model={unmeasured} />);

    expect(
      screen.queryByTestId("settings-page-skills-source-agents-count")
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("settings-page-skills-source-agents-toggle")).toBeEnabled();
    expect(
      screen.queryByTestId("settings-page-skills-source-team-skills-unreadable")
    ).not.toBeInTheDocument();
  });

  it("Should mark a truncated folder and keep its real count", async () => {
    const user = userEvent.setup();
    const truncated = withSources(sources => {
      const root = sources[3].roots[0];
      root.truncated = true;
      root.scanned_count = 300;
      root.skill_count = 214;
    });
    render(<SettingsSkillSourcesSection model={truncated} />);

    expect(
      screen.getByTestId("settings-page-skills-source-team-skills-truncated")
    ).toHaveTextContent("partial scan");
    await user.click(screen.getByTestId("settings-page-skills-source-team-skills-disclosure"));
    const rootId = "settings-page-skills-source-team-skills-root-r_u_custom_a41c";
    expect(screen.getByTestId(`${rootId}-state`)).toHaveTextContent(
      "large folder — first 300 scanned"
    );
    expect(screen.getByTestId(`${rootId}-count`)).toHaveTextContent("214 skills");
  });

  it("Should explain a scanned-versus-usable gap with the daemon's own diagnostics", async () => {
    const user = userEvent.setup();
    render(<SettingsSkillSourcesSection model={model()} />);

    await user.click(screen.getByTestId("settings-page-skills-source-agents-disclosure"));
    const rootId = "settings-page-skills-source-agents-root-r_w_agents_9f31";
    expect(screen.getByTestId(`${rootId}-count`)).toHaveTextContent("5 found · 3 usable");

    await user.click(screen.getByRole("button", { name: "Why only 3 of 5?" }));
    const diagnostics = screen.getByTestId(`${rootId}-diagnostics`);
    expect(within(diagnostics).getByTestId(`${rootId}-diagnostics-skipped`)).toHaveTextContent(
      "review-old"
    );
    expect(within(diagnostics).getByTestId(`${rootId}-diagnostics-collisions`)).toHaveTextContent(
      "agents:frontend-qa"
    );
    expect(within(diagnostics).getByTestId(`${rootId}-diagnostics-verification`)).toHaveTextContent(
      "1 warning · nothing blocked"
    );
  });

  it("Should present defaults only when nothing optional is on", () => {
    render(
      <SettingsSkillSourcesSection
        model={model({
          enabledPresets: [],
          customEntries: [],
          groups: groupSkillSources(
            structuredClone(settingsSkillSourcesFixture)
              .filter(source => source.kind !== "custom")
              .map(source =>
                source.always_on ? source : { ...source, enabled: false }
              ) as SettingsSkillSource[]
          ),
        })}
      />
    );

    expect(screen.getByTestId("settings-page-skills-sources-defaults-only")).toHaveTextContent(
      "Defaults only"
    );
  });

  it("Should render the daemon's rejection verbatim and keep the draft", () => {
    render(
      <SettingsSkillSourcesSection
        model={model({
          isDirty: true,
          saveError: 'unknown skill source preset "cluade"',
          saveErrorCode: "unknown_skill_source",
        })}
      />
    );

    const error = screen.getByTestId("settings-page-skills-sources-save-error");
    expect(error).toHaveTextContent('unknown skill source preset "cluade"');
    expect(error).toHaveTextContent("unknown_skill_source");
    expect(screen.getByTestId("settings-page-skills-sources-save")).toBeEnabled();
  });

  it("Should state each key's inheritance at workspace scope", async () => {
    const user = userEvent.setup();
    const useInherited = vi.fn();
    const customize = vi.fn();
    render(
      <SettingsSkillSourcesSection
        model={model({
          useInherited,
          customize,
          postures: [
            { key: "sources", inherited: false, armed: true },
            { key: "custom_sources", inherited: true, armed: false },
          ],
        })}
      />
    );

    expect(
      screen.getByTestId("settings-page-skills-sources-key-sources-posture")
    ).toHaveTextContent("custom for this workspace");
    expect(
      screen.getByTestId("settings-page-skills-sources-key-custom_sources-posture")
    ).toHaveTextContent("inherited");

    await user.click(screen.getByTestId("settings-page-skills-sources-key-sources-use-inherited"));
    expect(useInherited).toHaveBeenCalledWith("sources");

    await user.click(
      screen.getByTestId("settings-page-skills-sources-key-custom_sources-customize")
    );
    expect(customize).toHaveBeenCalledWith("custom_sources");
  });

  it("Should read as policy-only at agent scope", () => {
    render(
      <SettingsSkillSourcesSection model={model({ readOnly: true, readOnlyReason: "agent" })} />
    );

    expect(screen.getByTestId("settings-page-skills-sources-read-only")).toBeVisible();
    expect(screen.queryByTestId("settings-page-skills-sources-controls")).not.toBeInTheDocument();
    expect(screen.getByTestId("settings-page-skills-source-agents-toggle")).toHaveAttribute(
      "aria-disabled",
      "true"
    );
  });

  it("Should explain that a repository profile projection is read-only", () => {
    render(
      <SettingsSkillSourcesSection
        model={model({ readOnly: true, readOnlyReason: "repository-profile" })}
      />
    );

    expect(screen.getByTestId("settings-page-skills-sources-read-only")).toHaveTextContent(
      "workspace projection follows the active profile"
    );
    expect(screen.queryByTestId("settings-page-skills-sources-controls")).not.toBeInTheDocument();
  });
});
