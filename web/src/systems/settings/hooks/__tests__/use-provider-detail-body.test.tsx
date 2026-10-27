// Suite: useProviderDetailBody
// Invariant: when the provider dialog flips between overview and form, the body keeps the previous
// view for the urgent render and shows the new one in a transition render (the render
// <ViewTransition> animates); every other change, typing included, is never deferred.
// Owner: provider dialog body view.
import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { providerDraftFromEntry } from "../../lib/provider-draft";
import { settingsProviderFixtures } from "../../mocks/fixtures";
import { useProviderDetailBody, type ProviderDetailBodyView } from "../use-provider-detail-body";

const provider = settingsProviderFixtures[0];
const draft = providerDraftFromEntry(provider);

const inspect: ProviderDetailBodyView = { mode: "inspect", provider, draft: null };
const edit: ProviderDetailBodyView = { mode: "edit", provider, draft };
const closed: ProviderDetailBodyView = { mode: "inspect", provider: null, draft: null };

function renderBody(initial: ProviderDetailBodyView) {
  const seen: ProviderDetailBodyView[] = [];
  const view = renderHook(
    ({ live }) => {
      const body = useProviderDetailBody(live);
      seen.push(body);
      return body;
    },
    { initialProps: { live: initial } }
  );
  return { seen, ...view };
}

describe("useProviderDetailBody", () => {
  it("Should keep the overview for the urgent render of a flip to the form, then show the form", () => {
    const { seen, rerender, result } = renderBody(inspect);
    seen.length = 0;

    rerender({ live: edit });

    expect(seen.map(body => body.mode)).toEqual(["inspect", "edit"]);
    expect(seen[0].draft).toBeNull();
    expect(result.current).toBe(edit);
  });

  it("Should keep the form for the urgent render of a flip back to the overview", () => {
    const { seen, rerender, result } = renderBody(edit);
    seen.length = 0;

    rerender({ live: inspect });

    expect(seen.map(body => body.mode)).toEqual(["edit", "inspect"]);
    expect(seen[0].draft).toBe(draft);
    expect(result.current).toBe(inspect);
  });

  it("Should never defer a change that keeps the mode, so typing is live", () => {
    const { seen, rerender } = renderBody(edit);
    seen.length = 0;
    const typed = { ...draft, name: `${draft.name}-typed` };

    rerender({ live: { mode: "edit", provider, draft: typed } });

    expect(seen.length).toBeGreaterThan(0);
    expect(seen.every(body => body.draft === typed)).toBe(true);
  });

  it("Should show a view at once when the dialog opens or closes", () => {
    const opening = renderBody(closed);
    opening.seen.length = 0;
    opening.rerender({ live: { mode: "create", provider: null, draft } });
    expect(opening.seen[0].mode).toBe("create");

    const closing = renderBody(edit);
    closing.seen.length = 0;
    closing.rerender({ live: closed });
    expect(closing.seen[0]).toBe(closed);
  });
});
