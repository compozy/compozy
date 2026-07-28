import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import type {
  NetworkParticipationDraft,
  NetworkParticipationStrategy,
} from "../../lib/network-participation";
import { NetworkParticipationFields } from "../network-participation-fields";

function renderFields(
  allowedStrategies: readonly [NetworkParticipationStrategy, ...NetworkParticipationStrategy[]]
) {
  function Harness() {
    const [draft, setDraft] = useState<NetworkParticipationDraft>({
      mode: "local",
      channelId: "",
      channelStrategy: "",
    });
    return (
      <NetworkParticipationFields
        allowedStrategies={allowedStrategies}
        onChange={setDraft}
        testIdPrefix="participation"
        value={draft}
      />
    );
  }

  return render(<Harness />);
}

describe("NetworkParticipationFields", () => {
  it("Should require a channel when a Session opts into named Live participation", () => {
    renderFields(["named"]);

    fireEvent.change(screen.getByTestId("participation-mode"), {
      target: { value: "live" },
    });

    expect(screen.getByTestId("participation-strategy")).toHaveValue("named");
    expect(screen.queryByRole("option", { name: "This task run" })).toBeNull();
    expect(screen.getByTestId("participation-channel")).toBeRequired();
    expect(screen.getByTestId("participation-channel")).toHaveAttribute("aria-invalid", "true");

    fireEvent.change(screen.getByTestId("participation-channel"), {
      target: { value: "release-room" },
    });
    expect(screen.getByTestId("participation-channel")).not.toHaveAttribute("aria-invalid");
  });

  it("Should hide caller-authored channel identity for a derived Task run strategy", () => {
    renderFields(["named", "run"]);

    fireEvent.change(screen.getByTestId("participation-mode"), {
      target: { value: "live" },
    });
    fireEvent.change(screen.getByTestId("participation-strategy"), {
      target: { value: "run" },
    });

    expect(screen.getByTestId("participation-strategy")).toHaveValue("run");
    expect(screen.queryByTestId("participation-channel")).toBeNull();
    expect(screen.queryByRole("option", { name: "This Loop run" })).toBeNull();
  });
});
