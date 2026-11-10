/** Turns one MDX source into renderable blocks: prose stays Markdown, the docs' own components become typed blocks. */

export interface LinkItem {
  href: string;
  label: string;
  title: string;
  description: string;
  meta: string;
}

export interface PathLinkItem {
  href: string;
  icon: string;
  text: string;
  cta: boolean;
}

export interface PathCardItem {
  label: string;
  title: string;
  description: string;
  links: PathLinkItem[];
}

export type Block =
  | { kind: "md"; id: string; text: string }
  | { kind: "note"; id: string; label: string; text: string }
  | { kind: "callout"; id: string; tone: string; text: string }
  | { kind: "links"; id: string; variant: "routes" | "guides"; items: LinkItem[] }
  | { kind: "workflow"; id: string; steps: { title: string; text: string }[] }
  | { kind: "diagram"; id: string; chart: string; caption: string }
  | { kind: "paths"; id: string; cards: PathCardItem[] }
  | { kind: "groups"; id: string; rows: { name: string; text: string }[] }
  | { kind: "openapi"; id: string };

export interface TocItem {
  id: string;
  text: string;
  depth: 2 | 3;
}

export interface Doc {
  blocks: Block[];
  toc: TocItem[];
  minutes: number;
  /** The Markdown body as authored, for "Copy as Markdown". */
  source: string;
}

/** The release the snapshot was taken at; production reads it from GitHub. */
const RELEASE = "v0.5.5";

const INSTALL: Record<string, string> = {
  "curl-pinned": `curl -fsSL https://compozy.com/install.sh | sh -s -- --version ${RELEASE} --dir "$HOME/.local/bin"`,
  "curl-env": `curl -fsSL https://compozy.com/install.sh |\n  COMPOZY_VERSION=${RELEASE} COMPOZY_INSTALL_DIR="$HOME/.local/bin" COMPOZY_SKIP_BOOTSTRAP=1 sh`,
  go: `go install github.com/compozy/compozy@${RELEASE}`,
};

const CONTAINERS = new Set([
  "OperatorNote",
  "Callout",
  "RouteList",
  "GuideGrid",
  "Workflow",
  "PathGrid",
  "GroupIndex",
  "Cards",
]);

export function slugify(text: string) {
  return text
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}

function attrs(tag: string) {
  const found: Record<string, string> = {};
  const pattern = /([A-Za-z]+)=(?:"([^"]*)"|'([^']*)'|\{`([\s\S]*?)`\})/g;
  for (const match of tag.matchAll(pattern)) {
    found[match[1] ?? ""] = match[2] ?? match[3] ?? match[4] ?? "";
  }
  return found;
}

function dedent(text: string) {
  const lines = text.replace(/^\n+|\s+$/g, "").split("\n");
  let cut = Number.POSITIVE_INFINITY;
  for (const line of lines) {
    if (line.trim()) cut = Math.min(cut, /^ */.exec(line)?.[0].length ?? 0);
  }
  if (!Number.isFinite(cut)) cut = 0;
  return lines.map(line => line.slice(cut)).join("\n");
}

function inner(element: string, name: string) {
  const open = element.indexOf(">");
  const close = element.lastIndexOf(`</${name}>`);
  return close > open ? element.slice(open + 1, close) : "";
}

function linkItems(element: string): LinkItem[] {
  const items: LinkItem[] = [];
  for (const match of element.matchAll(/<(?:RouteRow|GuideCard|Card)\b([\s\S]*?)\/>/g)) {
    const found = attrs(match[1] ?? "");
    items.push({
      href: found.href ?? "",
      label: found.label ?? "",
      title: found.title ?? "",
      description: found.description ?? "",
      meta: found.meta ?? "",
    });
  }
  return items;
}

function pathCards(element: string): PathCardItem[] {
  const cards: PathCardItem[] = [];
  for (const match of element.matchAll(/<PathCard\b([\s\S]*?)>\n([\s\S]*?)<\/PathCard>/g)) {
    const found = attrs(match[1] ?? "");
    const links: PathLinkItem[] = [];
    for (const link of (match[2] ?? "").matchAll(/<PathLink\s+([^>]*)>([\s\S]*?)<\/PathLink>/g)) {
      const linkAttrs = attrs(link[1] ?? "");
      links.push({
        href: linkAttrs.href ?? "",
        icon: linkAttrs.icon ?? "",
        text: (link[2] ?? "").trim(),
        cta: /\bcta\b/.test(link[1] ?? ""),
      });
    }
    cards.push({
      label: found.label ?? "",
      title: found.title ?? "",
      description: found.description ?? "",
      links,
    });
  }
  return cards;
}

function toBlock(name: string, element: string, id: string): Block | null {
  const opening = element.slice(0, element.indexOf(">") + 1);
  const found = attrs(name === "Mermaid" ? element : opening);
  switch (name) {
    case "OperatorNote":
      return { kind: "note", id, label: found.label ?? "Note", text: dedent(inner(element, name)) };
    case "Callout":
      return {
        kind: "callout",
        id,
        tone: found.type ?? "info",
        text: dedent(inner(element, name)),
      };
    case "RouteList":
      return { kind: "links", id, variant: "routes", items: linkItems(element) };
    case "GuideGrid":
    case "Cards":
      return { kind: "links", id, variant: "guides", items: linkItems(element) };
    case "Workflow": {
      const steps = [];
      const pattern = /<WorkflowStep\s+title="([^"]*)"\s*>([\s\S]*?)<\/WorkflowStep>/g;
      for (const match of element.matchAll(pattern)) {
        steps.push({ title: match[1] ?? "", text: dedent(match[2] ?? "") });
      }
      return { kind: "workflow", id, steps };
    }
    case "PathGrid":
      return { kind: "paths", id, cards: pathCards(element) };
    case "GroupIndex": {
      const rows = [];
      for (const match of element.matchAll(
        /<GroupRow\s+name="([^"]*)"\s*>([\s\S]*?)<\/GroupRow>/g
      )) {
        rows.push({ name: match[1] ?? "", text: dedent(match[2] ?? "").replace(/\n/g, " ") });
      }
      return { kind: "groups", id, rows };
    }
    case "Mermaid":
      return { kind: "diagram", id, chart: found.chart ?? "", caption: found.caption ?? "" };
    case "InstallCommand":
      return {
        kind: "md",
        id,
        text: `\`\`\`bash\n${INSTALL[found.method ?? ""] ?? INSTALL.go}\n\`\`\``,
      };
    case "OpenAPIPage":
      return { kind: "openapi", id };
    default:
      return null;
  }
}

function plain(heading: string) {
  return heading
    .replace(/\[([^\]]+)\]\([^)]*\)/g, "$1")
    .replace(/[`*_]/g, "")
    .trim();
}

function tocOf(blocks: readonly Block[]) {
  const toc: TocItem[] = [];
  for (const block of blocks) {
    if (block.kind !== "md") continue;
    let fenced = false;
    for (const line of block.text.split("\n")) {
      if (line.startsWith("```")) fenced = !fenced;
      const match = fenced ? null : /^(#{2,3}) (.+)$/.exec(line);
      if (!match) continue;
      const text = plain(match[2] ?? "");
      toc.push({ id: slugify(text), text, depth: match[1] === "##" ? 2 : 3 });
    }
  }
  return toc;
}

/** Reads a whole element starting at `start`: a container to its closing tag, anything else to `/>`. */
function elementEnd(lines: readonly string[], start: number, name: string) {
  const closing = `</${name}>`;
  for (let at = start; at < lines.length; at += 1) {
    const line = lines[at] ?? "";
    if (CONTAINERS.has(name) ? line.includes(closing) : line.trimEnd().endsWith("/>")) return at;
  }
  return start;
}

export function parseDoc(raw: string): Doc {
  const source = raw
    .replace(/^---\n[\s\S]*?\n---\n/, "")
    .replace(/<CurrentReleaseTag \/>/g, RELEASE);
  const lines = source.split("\n");
  const blocks: Block[] = [];
  let prose: string[] = [];
  let fenced = false;
  const flush = () => {
    const text = prose.join("\n").trim();
    if (text) blocks.push({ kind: "md", id: `md-${blocks.length}`, text });
    prose = [];
  };
  for (let at = 0; at < lines.length; at += 1) {
    const line = lines[at] ?? "";
    if (line.trimStart().startsWith("```")) fenced = !fenced;
    const name = fenced ? undefined : /^<([A-Z][A-Za-z]*)/.exec(line)?.[1];
    if (!name) {
      prose.push(line);
      continue;
    }
    const end = elementEnd(lines, at, name);
    const block = toBlock(name, lines.slice(at, end + 1).join("\n"), `${name}-${blocks.length}`);
    if (block) {
      flush();
      blocks.push(block);
    }
    at = end;
  }
  flush();
  const words = source.split(/\s+/).length;
  return { blocks, toc: tocOf(blocks), minutes: Math.max(1, Math.round(words / 220)), source };
}
