import { render, screen, within } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import HomeLayout from "@/app/(home)/layout";
import BlogLayout from "@/app/blog/layout";
import ChangelogLayout from "@/app/changelog/layout";
import MarketplaceLayout from "@/app/marketplace/layout";

vi.mock("next/navigation", () => ({
  usePathname: () => "/",
}));

vi.mock("@/components/site/github-stars-api", () => ({
  loadGithubStars: vi.fn().mockResolvedValue(null),
}));

const LAYOUTS: ReadonlyArray<[string, (props: { children: ReactNode }) => ReactNode]> = [
  ["home", HomeLayout],
  ["blog", BlogLayout],
  ["changelog", ChangelogLayout],
  ["marketplace", MarketplaceLayout],
];

describe("marketing section layouts", () => {
  it.each(LAYOUTS)(
    "renders the %s body between the marketing header and footer",
    (name, Layout) => {
      render(
        <Layout>
          <p>{`${name} body`}</p>
        </Layout>
      );

      const header = screen.getByRole("banner");
      const main = screen.getByRole("main");
      const footer = screen.getByRole("contentinfo");

      expect(within(header).getByRole("navigation", { name: "Site" })).toBeDefined();
      expect(within(main).getByText(`${name} body`)).toBeDefined();
      expect(main.id).toBe("main-content");
      expect(within(footer).getByRole("link", { name: "GitHub" })).toBeDefined();
      expect(header.compareDocumentPosition(main) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
      expect(main.compareDocumentPosition(footer) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    }
  );
});
