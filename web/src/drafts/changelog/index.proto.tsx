import "../landing/landing.css";

import { definePrototype, type PrototypeProps } from "@compozy/proto";
import { LazyMotion, domMax } from "motion/react";
import { useState } from "react";

import { SiteFooter } from "../landing/sections-close";
import { SiteHeader } from "../landing/sections-open";
import { ReleaseDetail } from "./detail";
import { ChangelogList, ChangelogUnavailable } from "./page";
import { findRelease } from "./releases";

export const meta = definePrototype({
  title: "Changelog redesign",
  spec: "landing-redesign",
  surface: "compozy.com/changelog",
  states: [
    { id: "list", label: "Stable releases" },
    { id: "betas", label: "Including betas" },
    { id: "search", label: "Search" },
    { id: "empty", label: "No results" },
    { id: "major", label: "Release with breaking changes" },
    { id: "patch", label: "Release with one note" },
    { id: "small", label: "Release without notes" },
    { id: "unavailable", label: "GitHub unavailable" },
  ],
});

interface View {
  betas: boolean;
  query: string;
  version: string | null;
}

const START: Record<string, View> = {
  list: { betas: false, query: "", version: null },
  betas: { betas: true, query: "", version: null },
  search: { betas: false, query: "worktree", version: null },
  empty: { betas: false, query: "fax machine", version: null },
  major: { betas: false, query: "", version: "v0.5.0" },
  patch: { betas: false, query: "", version: "v0.5.5" },
  small: { betas: false, query: "", version: "v0.4.2" },
};

function Changelog({ initialView }: { initialView: View }) {
  const [view, setView] = useState(initialView);
  const release = view.version ? findRelease(view.version) : undefined;
  const open = (version: string) => {
    setView(current => ({ ...current, version }));
    window.scrollTo({ top: 0 });
  };
  if (release) {
    return (
      <ReleaseDetail
        key={release.version}
        release={release}
        onBack={() => setView(current => ({ ...current, version: null }))}
        onOpen={open}
      />
    );
  }
  return (
    <ChangelogList
      showBetas={view.betas}
      query={view.query}
      onBetas={betas => setView(current => ({ ...current, betas }))}
      onQuery={query => setView(current => ({ ...current, query }))}
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
          {state === "unavailable" ? (
            <ChangelogUnavailable />
          ) : (
            <Changelog key={state} initialView={START[state] ?? START.list} />
          )}
        </main>
        <SiteFooter />
      </div>
    </LazyMotion>
  );
}
