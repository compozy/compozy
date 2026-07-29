import { render } from "@testing-library/react";
import { AlertTriangleIcon } from "lucide-react";
import { describe, expect, it } from "vitest";

import { Marker, MarkerMeta } from "../marker";

describe("Marker", () => {
  it("Should carry tone on the glyph only — the sentence stays neutral", () => {
    const { container } = render(
      <Marker tone="danger" icon={<AlertTriangleIcon />}>
        <b>Provider request failed</b> — the runtime is retrying.
      </Marker>
    );
    const marker = container.querySelector<HTMLElement>('[data-slot="marker"]');
    const icon = container.querySelector<HTMLElement>('[data-slot="marker-icon"]');
    const body = container.querySelector<HTMLElement>('[data-slot="marker-body"]');

    expect(marker).toHaveAttribute("data-tone", "danger");
    expect(icon?.className).toContain("text-danger");
    expect(body?.className).not.toContain("text-danger");
  });

  it("Should render without an icon and default to the neutral tone", () => {
    const { container } = render(<Marker>Session resumed.</Marker>);
    const marker = container.querySelector<HTMLElement>('[data-slot="marker"]');

    expect(marker).toHaveAttribute("data-tone", "neutral");
    expect(container.querySelector('[data-slot="marker-icon"]')).toBeNull();
    expect(marker?.textContent).toBe("Session resumed.");
  });

  it("Should render MarkerMeta as faint mono for kind strings and cluster counts", () => {
    const { container } = render(
      <Marker>
        Runtime event <MarkerMeta>agent.retry ×3</MarkerMeta>
      </Marker>
    );
    const metaNode = container.querySelector<HTMLElement>('[data-slot="marker-meta"]');

    expect(metaNode?.textContent).toBe("agent.retry ×3");
    expect(metaNode?.className).toContain("font-mono");
    expect(metaNode?.className).toContain("text-faint");
  });
});
