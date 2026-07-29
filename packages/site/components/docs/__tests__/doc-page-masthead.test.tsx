import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DocPageMasthead } from "../doc-page-masthead";

describe("DocPageMasthead", () => {
  it("renders context-row crumbs above the title without Focus filler", () => {
    render(
      <DocPageMasthead
        product="CompozyOS Runtime"
        audience="people running agent work"
        crumbs={[{ name: "Core Concepts", href: "/runtime/core" }, { name: "Sessions" }]}
        title="Session Lifecycle"
        description="Durable runtime unit."
        sectionPageCount={4}
      />
    );

    expect(screen.getByText("CompozyOS Runtime")).toBeTruthy();
    expect(screen.getByRole("link", { name: "Core Concepts" })).toBeTruthy();
    expect(screen.getByText("Sessions")).toBeTruthy();
    expect(screen.getByRole("heading", { level: 1, name: "Session Lifecycle" })).toBeTruthy();
    expect(screen.getByText("people running agent work")).toBeTruthy();
    expect(screen.getByText("4 pages")).toBeTruthy();
    expect(screen.queryByText(/guidance shaped for scanability/i)).toBeNull();
    expect(screen.queryByText("Focus")).toBeNull();
  });

  it("omits the page-count meta item when count is unavailable", () => {
    render(
      <DocPageMasthead
        product="CompozyOS Runtime"
        audience="people running agent work"
        crumbs={[{ name: "Runtime Overview" }]}
        title="Runtime Documentation"
        description="CompozyOS Runtime overview."
        sectionPageCount={null}
      />
    );

    expect(screen.getByText("Runtime Overview")).toBeTruthy();
    expect(screen.queryByText(/in this section/i)).toBeNull();
  });

  it("renders page actions when action urls are provided", () => {
    render(
      <DocPageMasthead
        product="Compozy Network"
        audience="protocol implementers"
        crumbs={[{ name: "Overview" }]}
        title="Protocol"
        markdownUrl="/llms.mdx/protocol/"
        pageUrl="https://compozy.com/protocol/"
        githubUrl="https://github.com/compozy/compozy"
      />
    );

    expect(screen.getByRole("button", { name: "Copy as Markdown" })).toBeTruthy();
    expect(screen.getByRole("button", { name: /Open with AI/i })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Page options" })).toBeTruthy();
  });
});
