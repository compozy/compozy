// Suite: skill expose panel state matrix.
// Invariant: repair affordances follow ownership — CompozyOS repairs links it
// created and never offers an action on a foreign entry — and every target the
// operation named is accounted for, including compensated ones.
// Owning layer: skill domain component.
// Canonical suite: this file (the panel has no other owner).
// Boundary IN: exposure views + the expose model. Boundary OUT: HTTP.
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement, ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

// The menu's own open/close is Base UI's contract, not this panel's; jsdom has
// no layer to portal into. What this suite owns is which targets are offered and
// what the confirm action sends.
vi.mock("@compozy/ui", async () => {
  const actual = await vi.importActual<typeof import("@compozy/ui")>("@compozy/ui");
  const React = await vi.importActual<typeof import("react")>("react");
  return {
    ...actual,
    DropdownMenu: ({ children }: { children: ReactNode }) =>
      React.createElement("div", null, children),
    DropdownMenuContent: ({ children, ...props }: { children: ReactNode }) =>
      React.createElement("div", props, children),
    DropdownMenuCheckboxItem: ({
      children,
      checked,
      onCheckedChange,
      ...props
    }: {
      children: ReactNode;
      checked: boolean;
      onCheckedChange: () => void;
    }) =>
      React.createElement(
        "button",
        {
          ...props,
          "aria-checked": checked,
          onClick: onCheckedChange,
          role: "menuitemcheckbox",
          type: "button",
        },
        children
      ),
    DropdownMenuLabel: ({ children }: { children: ReactNode }) =>
      React.createElement("div", null, children),
    DropdownMenuTrigger: ({ render: trigger }: { render?: ReactElement }) =>
      React.isValidElement(trigger) ? trigger : null,
  };
});

import type { SkillExposeModel } from "../../hooks/use-skill-expose";
import {
  isSkillExposable,
  skillExposeResultViews,
  toSkillExposureView,
} from "../../lib/skill-exposure-view";
import { skillExposePartialFailureFixture, skillExposuresFixture } from "../../mocks";
import type { SkillExposeResult, SkillPayload } from "../../types";
import { SkillExposePanel } from "../skill-expose-panel";
import type { SkillExposeTarget } from "../../types";

const TARGETS: SkillExposeTarget[] = [
  { slug: "agents", label: "Universal (.agents)", hint: ".agents/skills" },
  { slug: "claude", label: "Claude (.claude)", hint: ".claude/skills" },
];

function exposeModel(overrides: Partial<SkillExposeModel> = {}): SkillExposeModel {
  return {
    pendingTargets: [],
    isPending: false,
    results: [],
    failure: null,
    rolledBack: false,
    expose: vi.fn(),
    unexpose: vi.fn(),
    dismiss: vi.fn(),
    ...overrides,
    pendingAction: overrides.pendingAction ?? null,
  };
}

function renderPanel(
  options: {
    exposures?: typeof skillExposuresFixture;
    model?: Partial<SkillExposeModel>;
    targets?: SkillExposeTarget[];
    targetsLoading?: boolean;
    targetsError?: string | null;
    onRetryTargets?: () => void;
  } = {}
) {
  const model = exposeModel(options.model);
  render(
    <SkillExposePanel
      exposures={(options.exposures ?? []).map(toSkillExposureView)}
      labelForTarget={slug => TARGETS.find(target => target.slug === slug)?.label ?? slug}
      model={model}
      onRetryTargets={options.onRetryTargets}
      targets={options.targets ?? TARGETS}
      targetsError={options.targetsError}
      targetsLoading={options.targetsLoading}
    />
  );
  return model;
}

describe("SkillExposePanel", () => {
  it("Should offer the action and list nothing when the skill is exposed nowhere", () => {
    renderPanel();

    expect(screen.queryByTestId("skill-expose-panel-list")).not.toBeInTheDocument();
    expect(screen.getByTestId("skill-expose-target-picker-trigger")).toBeVisible();
  });

  it("Should report a healthy link as active with removal only", () => {
    renderPanel({ exposures: [skillExposuresFixture[0]] });

    const row = screen.getByTestId("skill-expose-panel-row-agents");
    expect(within(row).getByTestId("skill-expose-panel-row-agents-status")).toHaveTextContent(
      "active"
    );
    expect(within(row).getByTestId("skill-expose-panel-row-agents-unexpose")).toBeVisible();
    expect(
      within(row).queryByTestId("skill-expose-panel-row-agents-expose-again")
    ).not.toBeInTheDocument();
  });

  it("Should offer repair on a link we created that no longer works", async () => {
    const user = userEvent.setup();
    const model = renderPanel({
      exposures: [skillExposuresFixture[1], skillExposuresFixture[2]],
    });

    expect(screen.getByTestId("skill-expose-panel-row-claude-status")).toHaveTextContent(
      "the link was deleted"
    );
    expect(screen.getByTestId("skill-expose-panel-row-agents-status")).toHaveTextContent(
      "the skill's folder moved"
    );

    await user.click(screen.getByTestId("skill-expose-panel-row-claude-expose-again"));
    expect(model.expose).toHaveBeenCalledWith(["claude"]);
  });

  it("Should state a foreign entry and offer no action on it", () => {
    renderPanel({ exposures: [skillExposuresFixture[3]] });

    const row = screen.getByTestId("skill-expose-panel-row-claude");
    expect(row).toHaveTextContent("another app's file is there");
    expect(within(row).queryByRole("button")).not.toBeInTheDocument();
  });

  it("Should expose to the enabled presets the operator picked", async () => {
    const user = userEvent.setup();
    const model = renderPanel();

    await user.click(screen.getByTestId("skill-expose-target-picker-trigger"));
    await user.click(screen.getByTestId("skill-expose-target-picker-option-agents"));
    await user.click(screen.getByTestId("skill-expose-target-picker-option-claude"));
    await user.click(screen.getByTestId("skill-expose-target-picker-confirm"));

    expect(model.expose).toHaveBeenCalledWith(["agents", "claude"]);
  });

  it("Should offer no target when no source is turned on", () => {
    renderPanel({ targets: [] });

    expect(screen.getByTestId("skill-expose-target-picker-none")).toBeVisible();
    expect(screen.queryByTestId("skill-expose-target-picker-trigger")).not.toBeInTheDocument();
  });

  it("Should not call an unfinished source read an empty target list", () => {
    renderPanel({ targets: [], targetsLoading: true });

    expect(screen.getByTestId("skill-expose-target-picker-loading")).toHaveTextContent(
      "Loading sources"
    );
    expect(screen.queryByTestId("skill-expose-target-picker-none")).not.toBeInTheDocument();
  });

  it("Should show the source read failure and offer retry", async () => {
    const user = userEvent.setup();
    const retry = vi.fn();
    renderPanel({ targets: [], targetsError: "Source policy unavailable", onRetryTargets: retry });

    expect(screen.getByTestId("skill-expose-target-picker-error")).toHaveTextContent(
      "Source policy unavailable"
    );
    await user.click(screen.getByRole("button", { name: "Retry" }));
    expect(retry).toHaveBeenCalledOnce();
  });

  it("Should claim no status for a target still being written", () => {
    renderPanel({ model: { pendingTargets: ["agents"], isPending: true } });

    const pending = screen.getByTestId("skill-expose-panel-pending-Universal (.agents)");
    expect(pending).toHaveTextContent("exposing…");
    expect(pending).not.toHaveTextContent("active");
  });

  it("Should account for every target of a partial failure, compensation included", () => {
    renderPanel({
      model: {
        failure: skillExposePartialFailureFixture.error.message,
        rolledBack: true,
        results: skillExposeResultViews(
          skillExposePartialFailureFixture.results as SkillExposeResult[]
        ),
      },
    });

    const conflict = screen.getByTestId("skill-expose-panel-result-claude");
    expect(conflict).toHaveTextContent("a file with this name is already there");
    expect(conflict).toHaveTextContent("expose_name_conflict");

    const compensated = screen.getByTestId("skill-expose-panel-result-agents");
    expect(compensated).toHaveTextContent("completed, then undone");
    expect(compensated).toHaveTextContent("rolled_back");
    expect(screen.getByTestId("skill-expose-panel-failure")).toHaveTextContent(
      "The target that had finished was undone."
    );
  });

  it("Should distinguish a target skipped by preflight from compensation", () => {
    const results = skillExposeResultViews([
      {
        target: "agents",
        ok: false,
        error: {
          code: "expose_not_applied",
          message: 'exposure was not applied because target "claude" failed preflight',
        },
      },
      {
        target: "claude",
        ok: false,
        error: { code: "expose_name_conflict" },
      },
    ]);

    expect(results[0]).toMatchObject({
      code: "expose_not_applied",
      rolledBack: false,
      sentence: "not applied because another target failed preflight",
    });
    expect(results[1]).toMatchObject({ code: "expose_name_conflict", rolledBack: false });
  });
});

describe("isSkillExposable", () => {
  const skill = (overrides: Partial<SkillPayload>): SkillPayload =>
    ({
      name: "review-checklist",
      description: "",
      source: "workspace",
      origin: "",
      enabled: true,
      activation: { active: true },
      dir: "/repo/.compozy/skills/review-checklist",
      ...overrides,
    }) as SkillPayload;

  it("Should treat a bundled skill as not exposable", () => {
    expect(isSkillExposable(skill({ source: "bundled", dir: "" }))).toBe(false);
  });

  it("Should treat a profile-owned skill as not exposable in this release", () => {
    expect(isSkillExposable(skill({ source: "profile" }))).toBe(false);
    expect(isSkillExposable(skill({ source: "workspace_profile" }))).toBe(false);
  });

  it("Should treat a skill with an on-disk home as exposable", () => {
    expect(isSkillExposable(skill({}))).toBe(true);
  });
});
