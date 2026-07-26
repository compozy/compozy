import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { JsonViewer } from "../json-viewer";

describe("JsonViewer", () => {
  it("Should color keys, strings, numbers, and booleans distinctly", () => {
    const { container } = render(<JsonViewer value={{ name: "tasks", count: 12, ok: true }} />);
    const code = container.querySelector('[data-slot="json-viewer-code"]');
    expect(code).not.toBeNull();
    expect(container.querySelector('[data-slot="json-key"]')).not.toBeNull();
    expect(container.querySelector('[data-slot="json-string"]')).not.toBeNull();
    expect(container.querySelector('[data-slot="json-number"]')).not.toBeNull();
    expect(container.querySelector('[data-slot="json-boolean"]')).not.toBeNull();
  });
});
