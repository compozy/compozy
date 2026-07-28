import { render, screen } from "@testing-library/react";
import { Sparkles } from "lucide-react";
import { describe, expect, it } from "vitest";

import { FormSection } from "../form-section";

describe("FormSection", () => {
  it("Should move explanatory prose behind a named help trigger", () => {
    render(
      <FormSection help="Where sessions using this profile execute." title="Where does it run?">
        body
      </FormSection>
    );
    // The prose is reachable but not resident: a `(?)` button carries it so the
    // default read of the section stays quiet.
    expect(screen.getByRole("button", { name: "About where does it run" })).toBeInTheDocument();
    expect(
      screen.queryByText("Where sessions using this profile execute.")
    ).not.toBeInTheDocument();
  });

  it("Should keep runtime truth visible instead of hiding it in the trigger", () => {
    render(
      <FormSection description="Project runtime defaults will be used." title="Runtime">
        body
      </FormSection>
    );
    expect(screen.getByText("Project runtime defaults will be used.")).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("Should render an optional icon + right eyebrow", () => {
    render(
      <FormSection title="Scope" icon={Sparkles} rightLabel="optional">
        body
      </FormSection>
    );
    expect(screen.getByText("optional").dataset.slot).toBe("form-section-right-label");
  });
});
