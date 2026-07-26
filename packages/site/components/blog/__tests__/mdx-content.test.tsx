import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";

import { MdxContent } from "../mdx-content";

const sampleMdxCode = `
const { Fragment: _Fragment, jsx: _jsx, jsxs: _jsxs } = arguments[0];
function _createMdxContent(props) {
  const _components = {
    code: "code",
    h2: "h2",
    p: "p",
    pre: "pre",
    ...props.components,
  };
  return _jsxs(_Fragment, {
    children: [
      _jsx(_components.h2, { children: "Operator notes" }),
      "\\n",
      _jsxs(_components.p, {
        children: [
          "Run ",
          _jsx(_components.code, { children: "agh daemon start" }),
          " before opening the workplace.",
        ],
      }),
      "\\n",
      _jsx(_components.pre, {
        "data-language": "bash",
        children: _jsx(_components.code, {
          "data-language": "bash",
          children: "agh daemon start",
        }),
      }),
    ],
  });
}
return {
  default: function MDXContent(props = {}) {
    const { wrapper: MDXLayout } = props.components || {};
    return MDXLayout
      ? _jsx(MDXLayout, { ...props, children: _jsx(_createMdxContent, { ...props }) })
      : _createMdxContent(props);
  },
};
`;

describe("blog MdxContent", () => {
  it("renders generated MDX with the public blog component set", () => {
    render(<MdxContent code={sampleMdxCode} />);

    screen.getByRole("heading", { name: "Operator notes", level: 2 });
    expect(screen.getAllByText("agh daemon start").length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "Copy code" })).toBeDefined();
  });

  it("lets pages override a mapped MDX component without losing defaults", () => {
    function CustomH2({ children }: { children?: ReactNode }) {
      return <h2 data-testid="custom-heading">{children}</h2>;
    }

    render(<MdxContent code={sampleMdxCode} components={{ h2: CustomH2 }} />);

    expect(screen.getByTestId("custom-heading").textContent).toBe("Operator notes");
    expect(screen.getByRole("button", { name: "Copy code" })).toBeDefined();
  });
});
