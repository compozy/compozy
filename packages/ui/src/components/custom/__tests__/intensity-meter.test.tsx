import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { IntensityMeter } from "../intensity-meter";

describe("IntensityMeter", () => {
  it("Should render seven bars with the filled count in data-position", () => {
    const { container } = render(<IntensityMeter position={3} />);
    const meter = container.querySelector('[data-slot="intensity-meter"]');
    expect(meter).not.toBeNull();
    expect(meter?.getAttribute("data-position")).toBe("3");
    expect(meter?.querySelectorAll("[data-fill]").length).toBe(7);
    expect(meter?.querySelectorAll('[data-fill="on"]').length).toBe(3);
  });

  it("Should clamp the fill count to the seven-bar range", () => {
    const { container } = render(<IntensityMeter position={99} />);
    const meter = container.querySelector('[data-slot="intensity-meter"]');
    expect(meter?.getAttribute("data-position")).toBe("7");
    expect(meter?.querySelectorAll('[data-fill="on"]').length).toBe(7);
  });

  it("Should render every bar hollow when hollow is set", () => {
    const { container } = render(<IntensityMeter position={5} hollow />);
    const meter = container.querySelector('[data-slot="intensity-meter"]');
    expect(meter?.getAttribute("data-hollow")).toBe("true");
    expect(meter?.getAttribute("data-position")).toBe("0");
    expect(meter?.querySelectorAll('[data-fill="hollow"]').length).toBe(7);
  });
});
