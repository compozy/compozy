import { render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { latchGatewayTierForTest } from "@/test/gateway-tier";

import { SettingsPageFrame } from "../settings-page-frame";
import { SettingsSaveBar } from "../settings-save-bar";

function renderFrame(saveBar: React.ReactNode) {
  return render(
    <SettingsPageFrame
      description="Changes here apply to new sessions on this machine."
      meta={[{ key: "sessions", content: <span>3 active sessions</span> }]}
      saveBar={saveBar}
      slug="general"
    >
      <div data-testid="frame-body-content">content</div>
    </SettingsPageFrame>
  );
}

describe("SettingsPageFrame", () => {
  let unlatch: () => void;
  beforeEach(() => {
    // The frame gates the save bar and restart notice on the listener tier;
    // these tests own the local shape (save bar present when dirty).
    unlatch = latchGatewayTierForTest("local");
  });
  afterEach(() => unlatch());

  it("renders the subhead sentence, quiet meta, and body content", () => {
    renderFrame(null);

    const subhead = screen.getByTestId("settings-page-general-subhead");
    expect(subhead).toHaveTextContent("Changes here apply to new sessions on this machine.");
    expect(subhead).toHaveTextContent("3 active sessions");
    expect(screen.getByTestId("settings-page-general-body")).toContainElement(
      screen.getByTestId("frame-body-content")
    );
  });

  it("keeps the floating save bar inside the scroll body when dirty", () => {
    renderFrame(
      <SettingsSaveBar
        slug="general"
        state={{ kind: "dirty", warnings: [] }}
        onSave={() => {}}
        onReset={() => {}}
      />
    );

    const body = screen.getByTestId("settings-page-general-body");
    expect(within(body).getByTestId("settings-page-general-save-bar")).toBeInTheDocument();
  });

  it("renders no save bar band when the page is clean", () => {
    renderFrame(
      <SettingsSaveBar
        slug="general"
        state={{ kind: "idle" }}
        onSave={() => {}}
        onReset={() => {}}
      />
    );

    expect(screen.queryByTestId("settings-page-general-save-bar")).not.toBeInTheDocument();
  });

  it("draws no subhead band for a page whose controls speak for themselves", () => {
    // A page that supplies neither a sentence nor meta gets no band at all — an
    // empty one would leave a rule under the heading with nothing above it.
    render(
      <SettingsPageFrame slug="palette">
        <div data-testid="frame-body-content">content</div>
      </SettingsPageFrame>
    );

    expect(screen.queryByTestId("settings-page-palette-subhead")).not.toBeInTheDocument();
    expect(screen.getByTestId("frame-body-content")).toBeVisible();
  });

  it("keeps the band for meta alone and drops the leading separator", () => {
    render(
      <SettingsPageFrame
        meta={[{ key: "count", content: <span>2 saved layouts</span> }]}
        slug="palette"
      >
        <div>content</div>
      </SettingsPageFrame>
    );

    const subhead = screen.getByTestId("settings-page-palette-subhead");
    expect(subhead).toHaveTextContent("2 saved layouts");
    expect(subhead.querySelectorAll('[aria-hidden="true"]')).toHaveLength(0);
  });

  it("does not own the route title or publish topbar state", () => {
    renderFrame(null);

    expect(screen.queryByRole("heading", { level: 1 })).not.toBeInTheDocument();
  });

  it("omits the subhead when there is no consequence line and no meta", () => {
    render(
      <SettingsPageFrame slug="roles">
        <div data-testid="frame-body-content">content</div>
      </SettingsPageFrame>
    );

    expect(screen.queryByTestId("settings-page-roles-subhead")).not.toBeInTheDocument();
  });

  it("renders live meta without a restating page subtitle", () => {
    render(
      <SettingsPageFrame
        meta={[
          { key: "ready", content: <span>1 ready</span> },
          { key: "fresh", content: <span>updated now</span> },
        ]}
        slug="providers"
      >
        <div>content</div>
      </SettingsPageFrame>
    );

    const subhead = screen.getByTestId("settings-page-providers-subhead");
    expect(subhead).toHaveTextContent("1 ready");
    expect(subhead).toHaveTextContent("updated now");
    expect(subhead.querySelectorAll('[aria-hidden="true"]')).toHaveLength(1);
  });

  it("hides the save bar while the tier cannot execute settings writes, keeping the read view", () => {
    const unlatch = latchGatewayTierForTest("private");

    try {
      renderFrame(
        <SettingsSaveBar
          slug="general"
          state={{ kind: "dirty", warnings: [] }}
          onSave={() => {}}
          onReset={() => {}}
        />
      );

      // Reads stay intact; the write affordance is absent, never disabled.
      expect(screen.getByTestId("frame-body-content")).toBeInTheDocument();
      expect(screen.queryByTestId("settings-page-general-save-bar")).not.toBeInTheDocument();
    } finally {
      unlatch();
    }
  });
});
