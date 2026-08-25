"use client";

import { AlertTriangleIcon, CheckIcon, CopyIcon } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";

import type { CodeBlockThemeMode } from "../../lib/code-theme";
import type { HighlightedCodeToken } from "../../lib/shiki-highlighter";
import { cn } from "../../lib/utils";
import { Button } from "../button";
import { Eyebrow } from "./eyebrow";
import { useCodeBlock } from "./hooks/use-code-block";

export type { CodeBlockHighlightState } from "./hooks/use-code-block";

export interface CodeBlockProps extends Omit<React.ComponentProps<"div">, "children"> {
  caption?: string;
  code: string;
  copyable?: boolean;
  copiedLabel?: string;
  copyFailedLabel?: string;
  copyLabel?: string;
  density?: CodeBlockDensity;
  highlightLines?: readonly number[];
  language?: string;
  showLineNumbers?: boolean;
  showPrompt?: boolean;
  themeMode?: CodeBlockThemeMode;
  truncateLines?: number;
  wrapLines?: boolean;
}

export type CodeBlockDensity = "default" | "compact";

export interface CopyIconButtonProps extends Omit<React.ComponentProps<typeof Button>, "children"> {
  copiedLabel?: string;
  copiedToastLabel?: string;
  copyFailedLabel?: string;
  copyFailedToastLabel?: string;
  copyLabel?: string;
  value: string;
}

type CopyState = "idle" | "copied" | "failed";

const COPY_FEEDBACK_MS = 1500;
const EMPTY_LINE = "\u00A0";

async function writeClipboard(value: string): Promise<boolean> {
  if (typeof navigator === "undefined") {
    return false;
  }

  try {
    const clipboard = navigator.clipboard;
    if (!clipboard || typeof clipboard.writeText !== "function") {
      return false;
    }
    await clipboard.writeText(value);
    return true;
  } catch {
    return false;
  }
}

/**
 * Code block per DESIGN.md §4. Canvas-deep container, JetBrains Mono body on
 * the shared block-code tier (`--text-code-block`, same as JsonViewer),
 * Vitesse syntax highlighting, optional language eyebrow, and a ghost copy
 * button. `showPrompt` opts into the accent `$ ` shell prompt — reach for it
 * only when the block really is a terminal transcript.
 */
function CodeBlock({
  caption,
  code,
  language,
  showPrompt = false,
  copyable = true,
  copyLabel = "Copy to clipboard",
  copiedLabel = "Copied",
  copyFailedLabel = "Copy failed",
  density = "default",
  themeMode = "auto",
  truncateLines,
  showLineNumbers = false,
  highlightLines,
  wrapLines = false,
  className,
  ...props
}: CodeBlockProps) {
  const {
    clampedLines,
    displayLines,
    highlightedCode,
    highlightedLineNumbers,
    highlightState,
    normalizedLanguage,
    resolvedThemeName,
  } = useCodeBlock({ code, highlightLines, language, themeMode, truncateLines });
  const label = caption ?? language;

  return (
    <div
      data-slot="code-block"
      data-highlight-state={highlightState}
      data-language={normalizedLanguage ?? undefined}
      data-density={density}
      data-theme={resolvedThemeName}
      className={cn(
        "relative overflow-hidden border border-line",
        density === "compact" ? "rounded-sm bg-canvas" : "rounded-lg bg-rail",
        className
      )}
      {...props}
    >
      {label ? (
        <Eyebrow
          data-slot="code-block-language"
          className={cn(
            "absolute text-subtle",
            density === "compact" ? "top-2 left-3" : "top-3 left-5"
          )}
        >
          {label}
        </Eyebrow>
      ) : null}
      {copyable ? (
        <CopyIconButton
          value={code}
          copyLabel={copyLabel}
          copiedLabel={copiedLabel}
          copyFailedLabel={copyFailedLabel}
          className={cn(
            "absolute text-subtle hover:text-accent data-[copied=true]:text-success data-[copy-state=failed]:text-danger data-[copy-state=failed]:hover:text-danger",
            density === "compact" ? "top-1.5 right-1.5" : "top-2 right-2"
          )}
        />
      ) : null}
      <pre
        data-slot="code-block-pre"
        style={
          clampedLines ? ({ "--code-block-lines": clampedLines } as React.CSSProperties) : undefined
        }
        className={cn(
          "overflow-x-auto font-mono text-fg",
          "[scrollbar-width:thin] [scrollbar-color:var(--color-line-strong)_transparent]",
          "[&::-webkit-scrollbar]:h-1.5 [&::-webkit-scrollbar]:w-1.5",
          "[&::-webkit-scrollbar-track]:bg-transparent",
          "[&::-webkit-scrollbar-thumb]:rounded-pill [&::-webkit-scrollbar-thumb]:bg-line-strong",
          "text-code-block",
          density === "compact" ? "px-3 py-2" : "px-5 py-4",
          wrapLines ? "whitespace-pre-wrap break-words" : "whitespace-pre",
          label ? (density === "compact" ? "pt-7" : "pt-9") : null,
          copyable ? (density === "compact" ? "pr-10" : "pr-12") : null,
          clampedLines
            ? density === "compact"
              ? "max-h-[calc(var(--code-block-lines)*1.5em+1.5rem)] overflow-y-auto"
              : "max-h-[calc(var(--code-block-lines)*1.5em+2rem)] overflow-y-auto"
            : null
        )}
      >
        <code data-slot="code-block-code">
          {displayLines.map(({ id, line, lineNumber }, index) => {
            const tokens = highlightedCode?.[index]?.tokens;
            const withPrompt = showPrompt && shouldRenderPrompt(line);
            const isHighlightedLine = highlightedLineNumbers.has(lineNumber);

            return (
              <span
                key={id}
                data-slot="code-block-line"
                data-line-number={lineNumber}
                data-highlighted={isHighlightedLine ? "true" : undefined}
                className={cn(
                  "block min-h-[1.5em]",
                  showLineNumbers ? "grid grid-cols-[2.25rem_minmax(0,1fr)] gap-3" : null,
                  isHighlightedLine ? "-mx-2 rounded-xs bg-surface-glaze px-2" : null
                )}
              >
                {showLineNumbers ? (
                  <span
                    data-slot="code-block-line-number"
                    aria-hidden="true"
                    className="select-none text-right text-faint tabular-nums"
                  >
                    {lineNumber}
                  </span>
                ) : null}
                <span data-slot="code-block-line-content" className="min-w-0">
                  {withPrompt ? (
                    <span
                      data-slot="code-block-prompt"
                      aria-hidden="true"
                      className="text-accent select-none"
                    >
                      {"$ "}
                    </span>
                  ) : null}
                  {renderCodeLine(line, tokens)}
                </span>
              </span>
            );
          })}
        </code>
      </pre>
    </div>
  );
}

function CopyIconButton({
  className,
  copiedLabel = "Copied",
  copiedToastLabel = "Copied to clipboard",
  copyFailedLabel = "Copy failed",
  copyFailedToastLabel = "Couldn't copy to clipboard",
  copyLabel = "Copy to clipboard",
  onClick,
  value,
  variant = "ghost",
  size = "icon-xs",
  type = "button",
  ...props
}: CopyIconButtonProps) {
  const [copyState, setCopyState] = React.useState<CopyState>("idle");
  const copyFeedbackTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);

  React.useEffect(
    () => () => {
      if (copyFeedbackTimerRef.current) {
        clearTimeout(copyFeedbackTimerRef.current);
      }
    },
    []
  );

  const scheduleReset = () => {
    if (copyFeedbackTimerRef.current) {
      clearTimeout(copyFeedbackTimerRef.current);
    }
    copyFeedbackTimerRef.current = setTimeout(() => setCopyState("idle"), COPY_FEEDBACK_MS);
  };

  const handleCopy = async () => {
    const copied = await writeClipboard(value);
    const nextState: CopyState = copied ? "copied" : "failed";
    setCopyState(nextState);
    if (copied) {
      toast.success(copiedToastLabel);
    } else {
      toast.error(copyFailedToastLabel);
    }
    scheduleReset();
  };

  const ariaLabel =
    copyState === "copied" ? copiedLabel : copyState === "failed" ? copyFailedLabel : copyLabel;

  return (
    <Button
      {...props}
      type={type}
      variant={variant}
      size={size}
      aria-label={ariaLabel}
      aria-live="polite"
      data-slot="code-block-copy"
      data-copy-state={copyState}
      data-copied={copyState === "copied" ? "true" : undefined}
      onClick={event => {
        onClick?.(event);
        if (!event.defaultPrevented) void handleCopy();
      }}
      className={className}
    >
      {copyState === "copied" ? (
        <CheckIcon className="size-3" />
      ) : copyState === "failed" ? (
        <AlertTriangleIcon className="size-3" />
      ) : (
        <CopyIcon className="size-3" />
      )}
    </Button>
  );
}

function renderCodeLine(line: string, tokens?: HighlightedCodeToken[]) {
  if (tokens && tokens.length > 0) {
    return tokens.map((token, index) => (
      <span
        data-slot="code-block-token"
        key={`${index}:${token.content}`}
        style={codeTokenStyle(token)}
      >
        {token.content}
      </span>
    ));
  }

  return line.length > 0 ? line : EMPTY_LINE;
}

function codeTokenStyle(token: HighlightedCodeToken): React.CSSProperties | undefined {
  const style: React.CSSProperties = {};
  if (token.color) style.color = token.color;
  if (token.fontStyle) style.fontStyle = token.fontStyle;
  if (token.fontWeight) style.fontWeight = token.fontWeight;
  if (token.textDecorationLine) style.textDecorationLine = token.textDecorationLine;

  return Object.keys(style).length > 0 ? style : undefined;
}

function shouldRenderPrompt(line: string): boolean {
  if (line.length === 0) return false;
  if (/^\s/.test(line)) return false;
  if (line.startsWith("#")) return false;
  return true;
}

export { CodeBlock, CopyIconButton };
