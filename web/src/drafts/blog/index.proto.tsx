import "../landing/landing.css";
import "./blog.css";

import { definePrototype, type PrototypeProps } from "@compozy/proto";
import { LazyMotion, domMax } from "motion/react";
import { useState } from "react";

import { SiteFooter } from "../landing/sections-close";
import { SiteHeader } from "../landing/sections-open";
import { BlogIndex, type Filters } from "./index-page";
import { BlogPost } from "./post-page";
import { findPost } from "./posts";

export const meta = definePrototype({
  title: "Blog redesign",
  spec: "landing-redesign",
  surface: "compozy.com/blog",
  states: [
    { id: "index", label: "Front page" },
    { id: "category", label: "Category" },
    { id: "search", label: "Search" },
    { id: "empty", label: "No results" },
    { id: "post", label: "Post with code" },
    { id: "callouts", label: "Long post with callouts" },
    { id: "launch", label: "Short post" },
  ],
});

interface View {
  filters: Filters;
  slug: string | null;
}

const NONE: Filters = { category: null, topic: null, query: "" };

const START: Record<string, View> = {
  index: { filters: NONE, slug: null },
  category: { filters: { ...NONE, category: "runtime" }, slug: null },
  search: { filters: { ...NONE, query: "git" }, slug: null },
  empty: { filters: { ...NONE, query: "fax machine" }, slug: null },
  post: { filters: NONE, slug: "ai-agent-retries-idempotency" },
  callouts: { filters: NONE, slug: "graph-loop-editor-local-gateway" },
  launch: { filters: NONE, slug: "introducing-compozyos" },
};

function Blog({ initialView }: { initialView: View }) {
  const [view, setView] = useState(initialView);
  const post = view.slug ? findPost(view.slug) : undefined;
  const open = (slug: string) => {
    setView(current => ({ ...current, slug }));
    window.scrollTo({ top: 0 });
  };
  const browse = (filters: Filters) => {
    setView({ filters, slug: null });
    window.scrollTo({ top: 0 });
  };
  if (post) {
    return (
      <BlogPost
        key={post.slug}
        post={post}
        onBack={() => browse(view.filters)}
        onCategory={category => browse({ ...NONE, category })}
        onOpen={open}
      />
    );
  }
  return (
    <BlogIndex
      filters={view.filters}
      onFilters={filters => setView({ filters, slug: null })}
      onOpen={open}
    />
  );
}

export default function Prototype({ state }: PrototypeProps) {
  return (
    <LazyMotion features={domMax}>
      <div id="landing" className="dark bg-[#0a0a0a] font-sans text-fg antialiased">
        <SiteHeader />
        <main>
          <Blog key={state} initialView={START[state] ?? START.index} />
        </main>
        <SiteFooter />
      </div>
    </LazyMotion>
  );
}
