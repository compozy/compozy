import { spawnSync } from "node:child_process";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it } from "vitest";
import plugin from "../compozy-design-system.mjs";

const TEST_DIR = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = resolve(TEST_DIR, "../..");
const PLUGIN_PATH = join(REPO_ROOT, "lint-plugins/compozy-design-system.mjs");
const tempRoots = [];

afterEach(async () => {
  const roots = tempRoots.splice(0);
  await Promise.all(roots.map(root => rm(root, { recursive: true, force: true })));
});

function pluginReferenceFrom(root) {
  const ref = relative(root, PLUGIN_PATH).replaceAll("\\", "/");
  return ref.startsWith(".") ? ref : `./${ref}`;
}

function collectMessages(value) {
  if (!value) return [];
  if (Array.isArray(value)) {
    return value.flatMap(item => collectMessages(item));
  }
  if (typeof value === "object") {
    const ownMessage = typeof value.message === "string" ? [value.message] : [];
    const diagnosticMessage =
      value.diagnostic && typeof value.diagnostic.message === "string"
        ? [value.diagnostic.message]
        : [];
    const nested = [];
    for (const key of ["diagnostics", "errors", "warnings", "files"]) {
      if (Array.isArray(value[key])) {
        nested.push(...collectMessages(value[key]));
      }
    }
    return [...ownMessage, ...diagnosticMessage, ...nested];
  }
  return [];
}

function literal(value) {
  return { type: "Literal", value };
}

function template(...quasis) {
  return {
    type: "TemplateLiteral",
    quasis: quasis.map(value => ({ value: { cooked: value } })),
  };
}

function identifier(name) {
  return { type: "Identifier", name };
}

function call(callee, args = []) {
  return { type: "CallExpression", callee, arguments: args };
}

function logical(left, right) {
  return { type: "LogicalExpression", left, right };
}

function conditional(consequent, alternate) {
  return { type: "ConditionalExpression", consequent, alternate };
}

function jsxExpression(expression) {
  return { type: "JSXExpressionContainer", expression };
}

function jsxClassNameAttribute(value, elementName = "div") {
  const node = {
    type: "JSXAttribute",
    name: { name: "className" },
    value,
    parent: null,
  };
  node.parent = {
    type: "JSXOpeningElement",
    name: { type: "JSXIdentifier", name: elementName },
  };
  return node;
}

function importSpecifier(importedName) {
  return {
    type: "ImportSpecifier",
    imported: { type: "Identifier", name: importedName },
  };
}

function literalImportSpecifier(importedName) {
  return {
    type: "ImportSpecifier",
    imported: { type: "Literal", value: importedName },
  };
}

function runRule(ruleName, filename, invoke) {
  const reports = [];
  const rule = plugin.rules[ruleName];
  const visitor = rule.create({
    filename,
    options: [],
    report(descriptor) {
      reports.push(descriptor);
    },
  });
  invoke(visitor);
  return reports;
}

function runClassNameRule(ruleName, filename, value, elementName = "div") {
  return runRule(ruleName, filename, visitor => {
    if (visitor.JSXAttribute) {
      visitor.JSXAttribute(jsxClassNameAttribute(value, elementName));
    }
  });
}

function runImportRule(ruleName, filename, sourceValue, specifiers) {
  return runRule(ruleName, filename, visitor => {
    if (visitor.ImportDeclaration) {
      visitor.ImportDeclaration({
        type: "ImportDeclaration",
        source: { value: sourceValue },
        specifiers,
      });
    }
  });
}

async function runOxlint({ filename, source, rule }) {
  const root = await mkdtemp(join(tmpdir(), "agh-lint-plugin-"));
  tempRoots.push(root);

  const sourcePath = join(root, filename);
  await mkdir(dirname(sourcePath), { recursive: true });
  await writeFile(sourcePath, source, "utf8");

  const configPath = join(root, ".oxlintrc.json");
  await writeFile(
    configPath,
    JSON.stringify(
      {
        plugins: [],
        jsPlugins: [pluginReferenceFrom(root)],
        rules: {
          [rule]: "error",
        },
      },
      null,
      2
    ),
    "utf8"
  );

  const result = spawnSync(
    "bunx",
    [
      "oxlint",
      "--config",
      configPath,
      "--format",
      "json",
      "--no-ignore",
      "--threads",
      "1",
      sourcePath,
    ],
    {
      cwd: root,
      encoding: "utf8",
      env: { ...process.env, FORCE_COLOR: "0", NO_COLOR: "1" },
    }
  );

  if (result.error) {
    throw result.error;
  }

  const stdout = result.stdout.trim();
  let messages = [];
  if (stdout) {
    try {
      messages = collectMessages(JSON.parse(stdout));
    } catch (error) {
      throw new Error(
        `Failed to parse oxlint JSON output: ${error.message}\nSTDOUT:\n${stdout}\nSTDERR:\n${result.stderr}`
      );
    }
  }

  return {
    exitCode: result.status,
    messages,
    stderr: result.stderr,
    stdout,
  };
}

async function expectViolation(input, expectedMessagePart) {
  const result = await runOxlint(input);
  expect(result.exitCode).not.toBe(0);
  expect(result.messages.join("\n")).toContain(expectedMessagePart);
  return result;
}

async function expectAllowed(input) {
  const result = await runOxlint(input);
  expect(result.exitCode).toBe(0);
  expect(result.messages).toEqual([]);
  expect(result.stderr).toBe("");
}

describe("compozy-design-system lint plugin", () => {
  describe("rule visitors", () => {
    it("registers the design-system rules", () => {
      expect(Object.keys(plugin.rules).sort()).toEqual([
        "no-banned-imports",
        "no-design-glaze-rgba",
        "no-inline-design-tuples",
        "no-inline-eyebrow",
        "no-low-contrast-focus-ring",
        "prefer-bare-token-utility",
      ]);
    });

    it("flags low-contrast and sub-2px focus rings, allows the shared focus tokens and accent rings", () => {
      const src = "/repo/web/src/foo.tsx";
      // Low-contrast focus color → focusRingColor (ring-line-strong / ring-ring).
      expect(
        runClassNameRule(
          "no-low-contrast-focus-ring",
          src,
          literal("focus-visible:ring-line-strong")
        )
      ).toHaveLength(1);
      // ring-ring with an opacity modifier is still a focus-color violation.
      expect(
        runClassNameRule("no-low-contrast-focus-ring", src, literal("focus-visible:ring-ring/50"))
      ).toHaveLength(1);
      // Sub-2px width → focusRingWidth, including bracketed has-[…focus…] variants.
      expect(
        runClassNameRule("no-low-contrast-focus-ring", src, literal("focus-visible:ring-1"))
      ).toHaveLength(1);
      expect(
        runClassNameRule(
          "no-low-contrast-focus-ring",
          src,
          literal("has-[input:focus-visible]:ring-1")
        )
      ).toHaveLength(1);
      // Color + width in one className report once per messageId (seen-dedup).
      expect(
        runClassNameRule(
          "no-low-contrast-focus-ring",
          src,
          literal("focus-visible:ring-1 focus-within:ring-line-strong focus-visible:ring-1")
        )
      ).toHaveLength(2);
      // Walks cn()/logical/conditional wrappers like the other className rules.
      expect(
        runClassNameRule(
          "no-low-contrast-focus-ring",
          src,
          jsxExpression(logical(identifier("active"), literal("focus-visible:ring-line-strong")))
        )
      ).toHaveLength(1);
      // Allowed: shared focus tokens, accent ring at ≥2px, ring-0, and resting/hover rings.
      for (const allowed of [
        "focus-visible:shadow-focus-ring",
        "focus-visible:shadow-focus-inset",
        "focus-visible:ring-2 focus-visible:ring-accent",
        "focus-visible:ring-0",
        "focus-visible:ring-offset-0",
        "ring-1",
        "hover:ring-1",
      ]) {
        expect(runClassNameRule("no-low-contrast-focus-ring", src, literal(allowed))).toHaveLength(
          0
        );
      }
      // In scope: @agh/ui primitives are covered alongside web.
      expect(
        runClassNameRule(
          "no-low-contrast-focus-ring",
          "/repo/packages/ui/src/foo.tsx",
          literal("focus-visible:ring-line-strong")
        )
      ).toHaveLength(1);
      // In scope: packages/site is covered too — a sub-2px focus ring fails.
      expect(
        runClassNameRule(
          "no-low-contrast-focus-ring",
          "/repo/packages/site/components/foo.tsx",
          literal("focus-visible:ring-1")
        )
      ).toHaveLength(1);
      // ...and the corrected 2px accent ring on packages/site passes.
      expect(
        runClassNameRule(
          "no-low-contrast-focus-ring",
          "/repo/packages/site/components/foo.tsx",
          literal("focus-visible:ring-2 focus-visible:ring-accent")
        )
      ).toHaveLength(0);
      // Exempt: test/story files and non-frontend source.
      expect(
        runClassNameRule(
          "no-low-contrast-focus-ring",
          "/repo/web/src/foo.test.tsx",
          literal("focus-visible:ring-1")
        )
      ).toHaveLength(0);
      expect(
        runClassNameRule(
          "no-low-contrast-focus-ring",
          "/repo/server/foo.tsx",
          literal("focus-visible:ring-1")
        )
      ).toHaveLength(0);
    });

    it("walks no-inline-eyebrow literals, templates, calls, conditionals, and logical expressions", () => {
      expect(
        runClassNameRule(
          "no-inline-eyebrow",
          "/repo/web/src/foo.tsx",
          literal("font-mono uppercase tracking-mono")
        )
      ).toHaveLength(1);
      expect(
        runClassNameRule(
          "no-inline-eyebrow",
          "/repo/web/src/foo.tsx",
          jsxExpression(template("uppercase text-[11px]"))
        )
      ).toHaveLength(1);
      expect(
        runClassNameRule(
          "no-inline-eyebrow",
          "/repo/web/src/foo.tsx",
          jsxExpression(call(identifier("cn"), [literal("font-mono uppercase tracking-mono")]))
        )
      ).toHaveLength(1);
      expect(
        runClassNameRule(
          "no-inline-eyebrow",
          "/repo/web/src/foo.tsx",
          jsxExpression(conditional(literal("font-mono uppercase"), literal("text-sm")))
        )
      ).toHaveLength(1);
      expect(
        runClassNameRule(
          "no-inline-eyebrow",
          "/repo/web/src/foo.tsx",
          jsxExpression(logical(identifier("active"), literal("font-mono uppercase")))
        )
      ).toHaveLength(1);
      expect(
        runClassNameRule(
          "no-inline-eyebrow",
          "/repo/web/src/foo.tsx",
          literal("font-mono text-[11px] tracking-mono")
        )
      ).toHaveLength(1);
      expect(
        runClassNameRule(
          "no-inline-eyebrow",
          "/repo/web/src/foo.tsx",
          literal("font-mono text-[11px]")
        )
      ).toHaveLength(0);
    });

    it("reports no-inline-eyebrow on deleted utility-class literals (eyebrow-badge / eyebrow-micro)", () => {
      expect(
        runClassNameRule(
          "no-inline-eyebrow",
          "/repo/web/src/foo.tsx",
          literal("eyebrow-badge text-(--muted)")
        )
      ).toHaveLength(1);
      expect(
        runClassNameRule(
          "no-inline-eyebrow",
          "/repo/web/src/foo.tsx",
          literal("eyebrow-micro text-(--subtle)")
        )
      ).toHaveLength(1);
      // The canonical `eyebrow` utility on its own is fine.
      expect(
        runClassNameRule(
          "no-inline-eyebrow",
          "/repo/web/src/foo.tsx",
          literal("eyebrow text-(--muted)")
        )
      ).toHaveLength(0);
      // Word-boundary check — `eyebrows-badge` is not the deleted utility.
      expect(
        runClassNameRule(
          "no-inline-eyebrow",
          "/repo/web/src/foo.tsx",
          literal("eyebrows-badge text-(--muted)")
        )
      ).toHaveLength(0);
    });

    it("respects no-inline-eyebrow exemptions without exempting packages/ui", () => {
      expect(
        runClassNameRule(
          "no-inline-eyebrow",
          "/repo/web/src/foo.tsx",
          literal("font-mono uppercase tracking-mono"),
          "dt"
        )
      ).toHaveLength(0);
      expect(
        runClassNameRule(
          "no-inline-eyebrow",
          "/repo/web/src/foo.tsx",
          literal("font-mono uppercase tracking-mono"),
          "TableHead"
        )
      ).toHaveLength(0);
      expect(
        runClassNameRule(
          "no-inline-eyebrow",
          "/repo/packages/ui/src/components/foo.tsx",
          literal("font-mono uppercase tracking-mono")
        )
      ).toHaveLength(1);
    });

    it("reports no-design-glaze-rgba across frontend source", () => {
      expect(
        runClassNameRule(
          "no-design-glaze-rgba",
          "/repo/web/src/foo.tsx",
          literal("bg-[rgba(255,255,255,0.022)]")
        )
      ).toHaveLength(1);
      expect(
        runClassNameRule(
          "no-design-glaze-rgba",
          "/repo/packages/site/components/foo.tsx",
          literal("bg-[rgba(255,255,255,0.022)]")
        )
      ).toHaveLength(1);
      expect(
        runClassNameRule(
          "no-design-glaze-rgba",
          "/repo/web/src/components/design-system-showcase.tsx",
          literal("bg-[rgba(255,255,255,0.022)]")
        )
      ).toHaveLength(0);
    });

    it("reports no-banned-imports only for Loader2 family outside canonical owners", () => {
      expect(
        runImportRule("no-banned-imports", "/repo/web/src/foo.tsx", "lucide-react", [
          importSpecifier("Loader2"),
          importSpecifier("Loader2Icon"),
        ])
      ).toHaveLength(2);
      expect(
        runImportRule("no-banned-imports", "/repo/web/src/foo.tsx", "lucide-react", [
          importSpecifier("RefreshCw"),
        ])
      ).toHaveLength(0);
      expect(
        runImportRule(
          "no-banned-imports",
          "/repo/packages/site/components/foo.tsx",
          "lucide-react",
          [importSpecifier("Loader2")]
        )
      ).toHaveLength(1);
      expect(
        runImportRule(
          "no-banned-imports",
          "/repo/packages/ui/src/components/sonner.tsx",
          "lucide-react",
          [importSpecifier("Loader2Icon")]
        )
      ).toHaveLength(0);
      expect(
        runImportRule("no-banned-imports", "/repo/web/src/foo.tsx", "lucide-react", [
          literalImportSpecifier("Loader2"),
        ])
      ).toHaveLength(1);
      expect(
        runImportRule("no-banned-imports", "/repo/web/src/foo.tsx", "lucide-react", [
          { type: "ImportSpecifier" },
          { type: "ImportDefaultSpecifier", imported: { type: "Identifier", name: "Loader2" } },
        ])
      ).toHaveLength(0);
    });

    it("reports no-inline-design-tuples for the 22px page h1 tuple", () => {
      expect(
        runClassNameRule(
          "no-inline-design-tuples",
          "/repo/web/src/foo.tsx",
          jsxExpression(
            call(call(identifier("cva"), [literal("text-[22px] tracking-[-0.026em]")]), [])
          )
        )[0]?.messageId
      ).toBe("pageH1Tuple");
      expect(
        runClassNameRule(
          "no-inline-design-tuples",
          "/repo/packages/site/components/foo.tsx",
          literal("text-[22px] tracking-[-0.026em]")
        )[0]?.messageId
      ).toBe("pageH1Tuple");
      expect(
        runClassNameRule(
          "no-inline-design-tuples",
          "/repo/web/src/foo.tsx",
          literal("text-detail-h1 tracking-detail-h1")
        )
      ).toHaveLength(0);
    });
  });

  describe("no-inline-eyebrow", () => {
    const rule = "compozy-design-system/no-inline-eyebrow";

    it("reports inline eyebrow tuples in cn(...) className calls with the prop-less message", async () => {
      const result = await expectViolation(
        {
          filename: "web/src/foo.tsx",
          rule,
          source: `
            export function View() {
              return <span className={cn("font-mono uppercase tracking-mono")}>x</span>;
            }
          `,
        },
        "Inlined eyebrow tuple in className. Use <Eyebrow> from @agh/ui (Inter UC 11/600/-0.005em, single style"
      );

      const message = result.messages.join("\n");
      expect(message).not.toContain("case=");
      expect(message).not.toContain("size=");
      expect(message).not.toContain("family=");
    });

    it("reports inline eyebrow tuples in packages/ui source", async () => {
      await expectViolation(
        {
          filename: "packages/ui/src/components/foo.tsx",
          rule,
          source: `
            export function View() {
              return <span className="font-mono uppercase tracking-mono">x</span>;
            }
          `,
        },
        "Inlined eyebrow tuple in className. Use <Eyebrow> from @agh/ui"
      );
    });

    it("allows unrelated className strings", async () => {
      await expectAllowed({
        filename: "web/src/foo.tsx",
        rule,
        source: `
          export function View() {
            return <span className="text-sm">x</span>;
          }
        `,
      });
    });

    it("reports the deleted eyebrow-badge utility literal in JSX className", async () => {
      await expectViolation(
        {
          filename: "web/src/foo.tsx",
          rule,
          source: `
            export function View() {
              return <span className="eyebrow-badge text-(--muted)">x</span>;
            }
          `,
        },
        "Inlined eyebrow tuple in className. Use <Eyebrow> from @agh/ui"
      );
    });

    it("reports the deleted eyebrow-micro utility literal in JSX className", async () => {
      await expectViolation(
        {
          filename: "web/src/foo.tsx",
          rule,
          source: `
            export function View() {
              return <span className="eyebrow-micro text-(--subtle)">x</span>;
            }
          `,
        },
        "Inlined eyebrow tuple in className. Use <Eyebrow> from @agh/ui"
      );
    });

    it("does not flag the canonical eyebrow utility literal", async () => {
      await expectAllowed({
        filename: "web/src/foo.tsx",
        rule,
        source: `
          export function View() {
            return <span className="eyebrow text-(--muted)">x</span>;
          }
        `,
      });
    });
  });

  describe("no-design-glaze-rgba", () => {
    const rule = "compozy-design-system/no-design-glaze-rgba";

    it("reports inline glaze rgba classes in clsx(...) className calls", async () => {
      await expectViolation(
        {
          filename: "web/src/foo.tsx",
          rule,
          source: `
            export function View() {
              return <div className={clsx("bg-[rgba(255,255,255,0.022)]")}>x</div>;
            }
          `,
        },
        "Inline surface glaze rgba in className."
      );
    });

    it("reports inline glaze rgba classes in packages/site source", async () => {
      await expectViolation(
        {
          filename: "packages/site/components/foo.tsx",
          rule,
          source: `
            export function View() {
              return <div className="bg-[rgba(255,255,255,0.022)]">x</div>;
            }
          `,
        },
        "Inline surface glaze rgba in className."
      );
    });

    it("allows tokenized glaze classes", async () => {
      await expectAllowed({
        filename: "web/src/foo.tsx",
        rule,
        source: `
          export function View() {
            return <div className="bg-(--row-hover)">x</div>;
          }
        `,
      });
    });

    it("allows the design-system showcase exemption", async () => {
      await expectAllowed({
        filename: "web/src/components/design-system-showcase.tsx",
        rule,
        source: `
          export function DesignSystemShowcase() {
            return <div className="bg-[rgba(255,255,255,0.022)]">x</div>;
          }
        `,
      });
    });
  });

  describe("no-banned-imports", () => {
    const rule = "compozy-design-system/no-banned-imports";

    it("reports Loader2 imports from lucide-react in web source", async () => {
      await expectViolation(
        {
          filename: "web/src/foo.tsx",
          rule,
          source: `
            import { Loader2 } from "lucide-react";

            export const Icon = Loader2;
          `,
        },
        "Importing Loader2 from lucide-react is banned in frontend code."
      );
    });

    it("reports Loader2Icon imports from lucide-react in web source", async () => {
      await expectViolation(
        {
          filename: "web/src/foo.tsx",
          rule,
          source: `
            import { Loader2Icon } from "lucide-react";

            export const Icon = Loader2Icon;
          `,
        },
        "Importing Loader2Icon from lucide-react is banned in frontend code."
      );
    });

    it("reports Loader2 imports from lucide-react in packages/site source", async () => {
      await expectViolation(
        {
          filename: "packages/site/components/foo.tsx",
          rule,
          source: `
            import { Loader2 } from "lucide-react";

            export const Icon = Loader2;
          `,
        },
        "Importing Loader2 from lucide-react is banned in frontend code."
      );
    });

    it("allows canonical Spinner ownership in packages/ui", async () => {
      await expectAllowed({
        filename: "packages/ui/src/components/spinner.tsx",
        rule,
        source: `
          import { Loader2Icon } from "lucide-react";

          export const Icon = Loader2Icon;
        `,
      });
    });
  });

  describe("no-inline-design-tuples", () => {
    const rule = "compozy-design-system/no-inline-design-tuples";

    it("reports the 22px page-h1 tuple in cva(...) className calls", async () => {
      await expectViolation(
        {
          filename: "web/src/foo.tsx",
          rule,
          source: `
            export function View() {
              return <h1 className={cva("text-[22px] tracking-[-0.026em]")()}>Title</h1>;
            }
          `,
        },
        "Inline 22px page H1 tuple in className."
      );
    });

    it("reports the 22px page-h1 tuple in packages/site source", async () => {
      await expectViolation(
        {
          filename: "packages/site/components/foo.tsx",
          rule,
          source: `
            export function View() {
              return <h1 className="text-[22px] tracking-[-0.026em]">Title</h1>;
            }
          `,
        },
        "Inline 22px page H1 tuple in className."
      );
    });

    it("allows tokenized detail heading classes", async () => {
      await expectAllowed({
        filename: "web/src/foo.tsx",
        rule,
        source: `
          export function View() {
            return <h1 className="text-(--text-detail-h1) tracking-detail-h1">Title</h1>;
          }
        `,
      });
    });
  });

  describe("prefer-bare-token-utility", () => {
    const rule = "compozy-design-system/prefer-bare-token-utility";

    it("flags color arbitrary-value syntax that has a bare utility in @theme", async () => {
      await expectViolation(
        {
          filename: "web/src/foo.tsx",
          rule,
          source: `
            export function View() {
              return <div className="text-(--muted)">x</div>;
            }
          `,
        },
        "text-(--muted)"
      );
    });

    it("flags bare-token arbitrary syntax in packages/site source", async () => {
      await expectViolation(
        {
          filename: "packages/site/components/foo.tsx",
          rule,
          source: `
            export function View() {
              return <div className="text-(--muted)">x</div>;
            }
          `,
        },
        "text-(--muted)"
      );
    });

    it("flags surface-glaze arbitrary syntax", async () => {
      await expectViolation(
        {
          filename: "web/src/foo.tsx",
          rule,
          source: `
            export function View() {
              return <div className="bg-(--row-hover)">x</div>;
            }
          `,
        },
        "bg-(--row-hover)"
      );
    });

    it("allows runtime vars injected by Radix (anchor-width, available-height)", async () => {
      await expectAllowed({
        filename: "web/src/foo.tsx",
        rule,
        source: `
          export function View() {
            return <div className="w-(--anchor-width) max-h-(--available-height)">x</div>;
          }
        `,
      });
    });

    it("allows component-internal tokens kept in :root (modal width, PillGroup heights)", async () => {
      await expectAllowed({
        filename: "web/src/foo.tsx",
        rule,
        source: `
          export function View() {
            return <div className="w-(--width-modal-md) min-h-(--height-pill-group-segment-md)">x</div>;
          }
        `,
      });
    });

    it("does not run inside test or story files", async () => {
      await expectAllowed({
        filename: "web/src/__tests__/foo.test.tsx",
        rule,
        source: `
          export function View() {
            return <div className="text-(--muted)">x</div>;
          }
        `,
      });
    });

    it("allows the eyebrow utility's internal length:--text-eyebrow syntax", async () => {
      await expectAllowed({
        filename: "web/src/foo.tsx",
        rule,
        source: `
          export function View() {
            return <span className="text-(length:--text-eyebrow)">x</span>;
          }
        `,
      });
    });
  });
});
