import * as React from "react";
import { defaultUrlTransform } from "streamdown";

import { cn } from "../../lib/utils";

const SAFE_DISALLOWED_ELEMENTS = [
  "script",
  "iframe",
  "object",
  "embed",
  "form",
  "input",
  "button",
  "style",
  "link",
  "meta",
  "base",
  "svg",
  "math",
] as const;

function isExternalUrl(value: string): boolean {
  if (!value) return false;
  if (value.startsWith("//")) return true;
  return /^[a-z][a-z0-9+.-]*:/i.test(value);
}

function SafeImage({
  src,
  alt,
  width,
  height,
  title,
  className,
}: React.ImgHTMLAttributes<HTMLImageElement>) {
  const url = typeof src === "string" ? src : "";
  const altText = typeof alt === "string" && alt.length > 0 ? alt : "image";
  if (isExternalUrl(url)) {
    return (
      <span
        data-slot="markdown-image-fallback"
        className="text-muted italic"
      >{`[image: ${altText}]`}</span>
    );
  }
  return (
    <img
      data-slot="markdown-image"
      src={url}
      alt={altText}
      width={width}
      height={height}
      title={title}
      className={cn("max-w-full rounded", className)}
    />
  );
}

const SAFE_COMPONENT_OVERRIDES = {
  strong: "strong",
  em: "em",
  code: "code",
  kbd: "kbd",
  s: "s",
  del: "del",
  ins: "ins",
  mark: "mark",
  blockquote: "blockquote",
  img: SafeImage,
} as const;

const STREAMDOWN_SAFE_CONFIG = {
  skipHtml: true as const,
  disallowedElements: SAFE_DISALLOWED_ELEMENTS,
  urlTransform: defaultUrlTransform,
  controls: false as const,
  lineNumbers: false as const,
  components: SAFE_COMPONENT_OVERRIDES,
};

export { STREAMDOWN_SAFE_CONFIG };
