import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { HeaderSearchInput } from "@/components/site/header-search-input";
import { SiteSearchDialog, SiteSearchProvider } from "@/components/site/site-search";

type SearchDialogProps = {
  children?: ReactNode;
  isLoading?: boolean;
  onSearchChange: (query: string) => void;
  search: string;
};

type ChildrenProps = {
  children?: ReactNode;
};

type SearchListItem = {
  content: ReactNode;
  id: string;
};

const mocks = vi.hoisted(() => {
  const state = {
    enabled: true,
    search: "",
  };
  const setSearch = vi.fn((query: string) => {
    state.search = query;
  });

  return {
    state,
    setOpenSearch: vi.fn(),
    setSearch,
    useDocsSearch: vi.fn(() => ({
      search: state.search,
      setSearch,
      query: {
        isLoading: false,
        data: "empty" as const,
      },
    })),
  };
});

vi.mock("fumadocs-ui/contexts/search", () => ({
  useSearchContext: () => ({
    enabled: mocks.state.enabled,
    hotKey: [
      { display: "Cmd", key: "meta" },
      { display: "K", key: "k" },
    ],
    setOpenSearch: mocks.setOpenSearch,
  }),
}));

vi.mock("fumadocs-core/search/client", () => ({
  useDocsSearch: mocks.useDocsSearch,
}));

vi.mock("fumadocs-ui/contexts/i18n", () => ({
  useI18n: () => ({
    locale: undefined,
    text: {
      search: "Search",
      searchNoResult: "No results",
    },
  }),
  I18nLabel: ({ label }: { label: string }) => <span>{label}</span>,
}));

vi.mock("fumadocs-ui/components/dialog/search", () => ({
  SearchDialog: ({ children, onSearchChange, search }: SearchDialogProps) => (
    <div data-search={search} role="dialog">
      <input
        aria-label="Dialog search"
        value={search}
        onChange={event => onSearchChange(event.currentTarget.value)}
      />
      {children}
    </div>
  ),
  SearchDialogClose: () => <button type="button">ESC</button>,
  SearchDialogContent: ({ children }: ChildrenProps) => <section>{children}</section>,
  SearchDialogFooter: ({ children }: ChildrenProps) => <footer>{children}</footer>,
  SearchDialogHeader: ({ children }: ChildrenProps) => <header>{children}</header>,
  SearchDialogIcon: () => <span aria-hidden="true" />,
  SearchDialogInput: () => <input aria-label="Search" readOnly />,
  SearchDialogList: ({ items }: { items?: SearchListItem[] | null }) => (
    <ul data-testid="search-list">
      {(items ?? []).map(item => (
        <li key={item.id}>{item.content}</li>
      ))}
    </ul>
  ),
  SearchDialogOverlay: () => <div data-testid="search-overlay" />,
  TagsList: ({ children }: ChildrenProps) => <div>{children}</div>,
  TagsListItem: ({ children }: ChildrenProps) => <button type="button">{children}</button>,
}));

function renderSearchSurface() {
  render(
    <SiteSearchProvider>
      <HeaderSearchInput />
      <SiteSearchDialog open onOpenChange={() => {}} api="/api/search" />
    </SiteSearchProvider>
  );
}

describe("site search bridge", () => {
  beforeEach(() => {
    mocks.state.enabled = true;
    mocks.state.search = "";
    mocks.setOpenSearch.mockClear();
    mocks.setSearch.mockClear();
    mocks.useDocsSearch.mockClear();
  });

  it("opens the Fumadocs search dialog with the query typed in the header searchbox", async () => {
    renderSearchSurface();

    const input = screen.getByRole("searchbox", { name: "Search docs" }) as HTMLInputElement;
    fireEvent.change(input, { target: { value: "memory" } });

    expect(input.value).toBe("memory");
    expect(mocks.setOpenSearch).toHaveBeenCalledWith(true);
    await waitFor(() => expect(mocks.setSearch).toHaveBeenLastCalledWith("memory"));
  });

  it("keeps the header searchbox synchronized when the dialog query changes", async () => {
    renderSearchSurface();

    const headerInput = screen.getByRole("searchbox", {
      name: "Search docs",
    }) as HTMLInputElement;
    const dialogInput = screen.getByRole("textbox", {
      name: "Dialog search",
    }) as HTMLInputElement;

    fireEvent.change(dialogInput, { target: { value: "agent" } });

    expect(mocks.setSearch).toHaveBeenLastCalledWith("agent");
    await waitFor(() => expect(headerInput.value).toBe("agent"));
  });

  it("does not render the header searchbox when Fumadocs search is disabled", () => {
    mocks.state.enabled = false;

    render(
      <SiteSearchProvider>
        <HeaderSearchInput hideIfDisabled />
      </SiteSearchProvider>
    );

    expect(screen.queryByRole("searchbox", { name: "Search docs" })).toBeNull();
  });
});
