import "../landing/landing.css";

import { LazyMotion, domMax } from "motion/react";
import { definePrototype, type PrototypeProps } from "@compozy/proto";
import { useState } from "react";

import { SiteFooter } from "../landing/sections-close";
import { SiteHeader } from "../landing/sections-open";
import { Browser } from "./browser";
import { findEntry } from "./catalog";
import { FromTheApp, Publish, SourcesAndIncluded } from "./closing";
import { EntryDetail } from "./detail";
import { Featured } from "./featured";
import { MarketplaceHero } from "./hero";

export const meta = definePrototype({
  title: "Marketplace redesign",
  spec: "landing-redesign",
  surface: "compozy.com/marketplace",
  states: [
    { id: "browse", label: "Browse" },
    { id: "category", label: "Category" },
    { id: "search", label: "Search" },
    { id: "empty", label: "No results" },
    { id: "detail", label: "Entry" },
  ],
});

interface View {
  category: string | null;
  query: string;
  entry: string | null;
}

const START: Record<string, View> = {
  browse: { category: null, query: "", entry: null },
  category: { category: "data", query: "", entry: null },
  search: { category: null, query: "postgres", entry: null },
  empty: { category: null, query: "fax machine", entry: null },
  detail: { category: null, query: "", entry: "figma" },
};

function jumpToCatalog() {
  document.getElementById("catalog")?.scrollIntoView({ behavior: "smooth", block: "start" });
}

function Marketplace({ initialView }: { initialView: View }) {
  const [view, setView] = useState(initialView);
  const entry = view.entry ? findEntry(view.entry) : undefined;
  const open = (id: string) => {
    setView(current => ({ ...current, entry: id }));
    window.scrollTo({ top: 0 });
  };
  const browseCategory = (category: string | null) => {
    setView({ category, query: "", entry: null });
    requestAnimationFrame(jumpToCatalog);
  };
  const search = (query: string) => setView(current => ({ ...current, query }));

  if (entry) {
    return (
      <EntryDetail
        entry={entry}
        onBack={() => setView(current => ({ ...current, entry: null }))}
        onCategory={browseCategory}
        onOpen={open}
      />
    );
  }
  const browsing = view.category === null && view.query.trim() === "";
  return (
    <>
      <MarketplaceHero
        query={view.query}
        onQuery={query => {
          search(query);
          requestAnimationFrame(() => {
            jumpToCatalog();
            document
              .querySelector<HTMLInputElement>("#catalog input")
              ?.focus({ preventScroll: true });
          });
        }}
      />
      {browsing ? <Featured onOpen={open} /> : null}
      <Browser
        category={view.category}
        query={view.query}
        onCategory={browseCategory}
        onQuery={search}
        onOpen={open}
      />
      <FromTheApp />
      <SourcesAndIncluded />
      <Publish />
    </>
  );
}

export default function Prototype({ state }: PrototypeProps) {
  return (
    <LazyMotion features={domMax}>
      <div id="landing" className="dark bg-[#0a0a0a] font-sans text-fg antialiased">
        <SiteHeader />
        <main>
          <Marketplace key={state} initialView={START[state] ?? START.browse} />
        </main>
        <SiteFooter />
      </div>
    </LazyMotion>
  );
}
