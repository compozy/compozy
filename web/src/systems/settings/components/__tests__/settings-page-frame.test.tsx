import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SettingsPageFrame } from "../settings-page-frame";

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
  it("renders the subhead sentence, quiet meta, and body content", () => {
    renderFrame(null);

    expect(screen.queryByRole("heading", { level: 1 })).not.toBeInTheDocument();
    const subhead = screen.getByTestId("settings-page-general-subhead");
    expect(subhead).toHaveTextContent("Changes here apply to new sessions on this machine.");
    expect(subhead).toHaveTextContent("3 active sessions");
    expect(screen.getByTestId("settings-page-general-body")).toContainElement(
      screen.getByTestId("frame-body-content")
    );
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
});
