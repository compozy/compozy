import { spawnSync } from "node:child_process";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it } from "vitest";
import plugin, { parsePrimitiveNames } from "../ui-primitive-reuse.mjs";

const TEST_DIR = dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = resolve(TEST_DIR, "../..");
const PLUGIN_PATH = join(REPO_ROOT, "lint-plugins/ui-primitive-reuse.mjs");
const RULE = "compozy-ui-reuse/no-shadow-ui-primitive";
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

function runRule(filename, invoke) {
  const reports = [];
  const rule = plugin.rules["no-shadow-ui-primitive"];
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

function functionDeclaration(name) {
  return { type: "FunctionDeclaration", id: { type: "Identifier", name } };
}

function variableDeclarator(name, initType) {
  return {
    type: "VariableDeclarator",
    id: { type: "Identifier", name },
    init: initType ? { type: initType } : null,
  };
}

async function runOxlint({ filename, source }) {
  const root = await mkdtemp(join(tmpdir(), "agh-ui-reuse-"));
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
        rules: { [RULE]: "error" },
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

  return { exitCode: result.status, messages, stderr: result.stderr };
}

describe("compozy-ui-reuse lint plugin", () => {
  describe("parsePrimitiveNames", () => {
    it("Should collect PascalCase value exports and skip types, hooks, helpers, and constants", () => {
      const names = parsePrimitiveNames(
        [
          'export { Button } from "./components/button";',
          'export { buttonVariants } from "./components/button-variants";',
          'export { Pill, PillDot, type PillProps, type PillTone } from "./components/custom/pill";',
          'export type { CardProps, CardSize } from "./components/card";',
          'export {\n  Timeline,\n  type TimelineProps,\n} from "./components/custom/timeline";',
          'export { useDirection, DirectionProvider } from "./components/direction";',
          'export { AGH_CODE_THEMES, normalizeAghCodeLanguage } from "./lib/code-theme";',
          'export { StatusDot as RenamedDot } from "./components/custom/status-dot";',
        ].join("\n")
      );
      expect([...names].sort()).toEqual([
        "Button",
        "DirectionProvider",
        "Pill",
        "PillDot",
        "RenamedDot",
        "Timeline",
      ]);
    });

    it("Should ban the real @agh/ui surface contract component names", () => {
      const reports = runRule("/repo/web/src/systems/settings/components/panel.tsx", visitor => {
        for (const name of [
          "Section",
          "Timeline",
          "ToolCallRow",
          "ActionResultBanner",
          "OperationalLinksRow",
          "FieldLabel",
          "Empty",
        ]) {
          visitor.FunctionDeclaration(functionDeclaration(name));
        }
      });
      expect(reports).toHaveLength(7);
    });
  });

  describe("rule visitors", () => {
    it("Should register exactly the no-shadow-ui-primitive rule", () => {
      expect(Object.keys(plugin.rules)).toEqual(["no-shadow-ui-primitive"]);
    });

    it("Should report function, class, and component-initialized const declarations", () => {
      const reports = runRule("/repo/web/src/routes/_app/vault.tsx", visitor => {
        visitor.FunctionDeclaration(functionDeclaration("ActionResultBanner"));
        visitor.ClassDeclaration({
          type: "ClassDeclaration",
          id: { type: "Identifier", name: "Empty" },
        });
        visitor.VariableDeclarator(variableDeclarator("ToolCallRow", "CallExpression"));
        visitor.VariableDeclarator(variableDeclarator("Section", "ArrowFunctionExpression"));
      });
      expect(reports).toHaveLength(4);
      expect(reports[0].data).toEqual({ name: "ActionResultBanner" });
    });

    it("Should ignore data-initialized consts and non-colliding names", () => {
      const reports = runRule("/repo/web/src/routes/_app/vault.tsx", visitor => {
        visitor.VariableDeclarator(variableDeclarator("Section", "MemberExpression"));
        visitor.VariableDeclarator(variableDeclarator("Section", null));
        visitor.FunctionDeclaration(functionDeclaration("VaultLastActionAlert"));
        visitor.FunctionDeclaration({ type: "FunctionDeclaration", id: null });
      });
      expect(reports).toEqual([]);
    });

    it("Should exempt tests, stories, storybook, the ui package, and non-consumer paths", () => {
      const exemptPaths = [
        "/repo/web/src/systems/session/components/__tests__/tool-call-card.test.tsx",
        "/repo/web/src/systems/network/components/stories/timeline.stories.tsx",
        "/repo/web/src/foo.stories.ts",
        "/repo/web/.storybook/preview.tsx",
        "/repo/packages/ui/src/components/custom/section.tsx",
        "/repo/sdk/typescript/src/section.tsx",
      ];
      for (const filename of exemptPaths) {
        const reports = runRule(filename, visitor => {
          if (visitor.FunctionDeclaration) {
            visitor.FunctionDeclaration(functionDeclaration("Section"));
          }
        });
        expect(reports, filename).toEqual([]);
      }
    });
  });

  describe("oxlint end-to-end", () => {
    it("Should fail a web consumer redefining a primitive as a function declaration", async () => {
      const result = await runOxlint({
        filename: "web/src/systems/settings/components/panel.tsx",
        source: "function Section() {\n  return null;\n}\nexport { Section };\n",
      });
      expect(result.exitCode).not.toBe(0);
      expect(result.messages.join("\n")).toContain('"Section" is an @agh/ui primitive');
    });

    it("Should fail a site consumer redefining a primitive through a wrapper call", async () => {
      const result = await runOxlint({
        filename: "packages/site/components/blog/chip.tsx",
        source:
          'import { memo } from "react";\nexport const KindChip = memo(function Chip() {\n  return null;\n});\n',
      });
      expect(result.exitCode).not.toBe(0);
      expect(result.messages.join("\n")).toContain('"KindChip" is an @agh/ui primitive');
    });

    it("Should allow stories, domain-prefixed names, and non-consumer paths", async () => {
      const allowed = [
        {
          filename: "web/src/systems/settings/routes/sandbox.stories.tsx",
          source: "export const Empty = () => null;\n",
        },
        {
          filename: "web/src/systems/session/components/session-tool-call-row.tsx",
          source: "export function SessionToolCallRow() {\n  return null;\n}\n",
        },
        {
          filename: "sdk/typescript/src/section.tsx",
          source: "export function Section() {\n  return null;\n}\n",
        },
      ];
      for (const input of allowed) {
        const result = await runOxlint(input);
        expect(result.exitCode, input.filename).toBe(0);
        expect(result.messages, input.filename).toEqual([]);
      }
    });
  });
});
