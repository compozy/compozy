const ASSET_ATTRIBUTES: ReadonlyArray<readonly [selector: string, attribute: string]> = [
  ["img[src], source[src], video[src], audio[src], track[src], input[src]", "src"],
  ["video[poster]", "poster"],
  ["link[href]", "href"],
];

const STRIPPED =
  "script, noscript, iframe, frame, frameset, object, embed, applet, portal, base, meta[http-equiv], link[rel~='import'], link[rel~='prefetch'], link[rel~='preload'], form";
const INLINE_HANDLER = /^on/i;
// A quoted value runs to its closing quote; an unquoted one stops at quotes, parentheses and
// whitespace, as in the CSS grammar, so unclosed `url(` tokens cannot make the scan quadratic.
const CSS_URL = /url\(\s*(?:"([^"]*)"|'([^']*)'|([^'"()\s]*))\s*\)/gi;

/** Serializes a URL as a double-quoted CSS string. */
function cssString(value: string): string {
  return `"${value.replace(/["\\\n]/g, char => (char === "\n" ? "\\a " : `\\${char}`))}"`;
}

/** Rewrites the `url()` references of a style sheet or `style` attribute through `resolve`. */
function rewriteCssUrls(css: string, baseUrl: string, resolve: (url: string) => string): string {
  return css.replace(
    CSS_URL,
    (match, double: string | undefined, single: string | undefined, bare: string | undefined) => {
      const trimmed = (double ?? single ?? bare ?? "").trim();
      if (trimmed === "" || trimmed.startsWith("data:") || trimmed.startsWith("#")) return match;
      try {
        return `url(${cssString(resolve(new URL(trimmed, baseUrl).href))})`;
      } catch {
        return "none";
      }
    }
  );
}

function originOf(url: string | undefined): string | null {
  if (!url) return null;
  try {
    return new URL(url).origin;
  } catch {
    return null;
  }
}

/**
 * The document the frame shows: the authored HTML with scripts, inline handlers, nested frames,
 * forms and refreshes removed, a `<base>` for relative URLs, and a Content-Security-Policy that
 * blocks scripts and every load except data URLs and `baseUrl`'s origin.
 */
export function prepareHtmlPreview(
  html: string,
  baseUrl?: string,
  resolveAssetUrl?: (url: string) => string
): string {
  const document = new DOMParser().parseFromString(html, "text/html");
  document.querySelectorAll(STRIPPED).forEach(node => node.remove());
  for (const element of Array.from(document.querySelectorAll("*"))) {
    for (const attribute of Array.from(element.attributes)) {
      if (INLINE_HANDLER.test(attribute.name)) element.removeAttribute(attribute.name);
    }
  }
  const origin = originOf(baseUrl);
  const sources = origin ? `data: ${origin}` : "data:";
  const policy = document.createElement("meta");
  policy.httpEquiv = "Content-Security-Policy";
  policy.content = [
    "default-src 'none'",
    "script-src 'none'",
    `img-src ${sources}`,
    `media-src ${sources}`,
    `font-src ${sources}`,
    `style-src 'unsafe-inline' ${sources}`,
    "form-action 'none'",
  ].join("; ");
  document.head.prepend(policy);
  if (origin && baseUrl) {
    const base = document.createElement("base");
    base.href = baseUrl;
    policy.after(base);
    if (resolveAssetUrl) {
      for (const [selector, attribute] of ASSET_ATTRIBUTES) {
        for (const element of Array.from(document.querySelectorAll(selector))) {
          const value = element.getAttribute(attribute);
          if (value === null || value.startsWith("data:")) continue;
          try {
            element.setAttribute(attribute, resolveAssetUrl(new URL(value, baseUrl).href));
          } catch {
            element.removeAttribute(attribute);
          }
        }
      }
      // Backgrounds and fonts referenced from CSS take the same route as element sources.
      for (const style of Array.from(document.querySelectorAll("style"))) {
        style.textContent = rewriteCssUrls(style.textContent ?? "", baseUrl, resolveAssetUrl);
      }
      for (const element of Array.from(document.querySelectorAll("[style]"))) {
        const css = element.getAttribute("style") ?? "";
        element.setAttribute("style", rewriteCssUrls(css, baseUrl, resolveAssetUrl));
      }
      // `srcset` candidates would bypass the rewrite; `src` stays the image.
      document.querySelectorAll("[srcset]").forEach(node => node.removeAttribute("srcset"));
    }
  }
  return `<!doctype html>\n${document.documentElement.outerHTML}`;
}
