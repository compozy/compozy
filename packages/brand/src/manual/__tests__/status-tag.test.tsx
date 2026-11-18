// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { pageBySlug } from "../chapters";
import { StatusTag } from "../parts/status-tag";

afterEach(cleanup);

describe("StatusTag", () => {
  it("labels a proposal chapter as Proposal", () => {
    render(<StatusTag status={pageBySlug("visuals").status} />);
    const tag = screen.getByText("Proposal");
    expect(tag.closest("[data-status]")?.getAttribute("data-status")).toBe("proposal");
    expect(screen.queryByText("Approved")).toBeNull();
  });

  it("labels an approved chapter as Approved", () => {
    render(<StatusTag status={pageBySlug("library").status} />);
    const tag = screen.getByText("Approved");
    expect(tag.closest("[data-status]")?.getAttribute("title")).toBe("Approved: binding rule");
  });
});
