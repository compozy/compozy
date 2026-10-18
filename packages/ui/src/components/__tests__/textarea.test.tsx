import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { Textarea } from "../textarea";

describe("Textarea", () => {
  it("Should forward rows and defaultValue", () => {
    render(<Textarea aria-label="notes" rows={5} defaultValue="hello" />);
    const textarea = screen.getByLabelText("notes") as HTMLTextAreaElement;
    expect(textarea.rows).toBe(5);
    expect(textarea.value).toBe("hello");
  });

  it("Should support controlled value + onChange", () => {
    function Harness() {
      const [value, setValue] = useState("");
      return (
        <Textarea
          aria-label="notes"
          value={value}
          onChange={event => setValue(event.currentTarget.value)}
        />
      );
    }
    render(<Harness />);
    const textarea = screen.getByLabelText("notes") as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: "new" } });
    expect(textarea.value).toBe("new");
  });

  it("Should apply aria-invalid styling when invalid", () => {
    const { container } = render(<Textarea aria-label="notes" aria-invalid defaultValue="" />);
    const textarea = container.querySelector('[data-slot="textarea"]');
    expect(textarea?.getAttribute("aria-invalid")).toBe("true");
  });

  it("Should respect the disabled attribute", () => {
    render(<Textarea aria-label="notes" disabled defaultValue="archived" />);
    const textarea = screen.getByLabelText("notes") as HTMLTextAreaElement;
    expect(textarea.disabled).toBe(true);
  });
});
