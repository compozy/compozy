"use client";

import { AlertTriangleIcon, CheckIcon, CopyIcon } from "lucide-react";
import * as React from "react";

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
  tone?: CodeBlockTone;
  truncateLines?: number;
  wrapLines?: boolean;
}

export type CodeBlockDensity = "default" | "compact";
export type CodeBlockTone = "default" | "warning" | "danger" | "success" | "info" | "accent";

export interface CopyIconButtonProps extends Omit<React.ComponentProps<typeof Button>, "children"> {
  copiedLabel?: string;
  copyFailedLabel?: string;
  copyLabel?: string;
  value: string;
}

type CopyState = "idle" | "copied" | "failed";

const COPY_FEEDBACK_MS = 1500;
const EMPTY_LINE = "\u00A0";

/**
 * Terminal-style code block per DESIGN.md §4. Canvas-deep container, JetBrains
 * Mono body at 14px/1.6, optional accent `$ ` prompt, Vitesse syntax
 * highlighting, optional language eyebrow, and a ghost copy button.
 */
function CodeBlock({
  caption,
  code,
  language,
  showPrompt = true,
  copyable = true,
  copyLabel = "Copy to clipboard",
  copiedLabel = "Copied",
  copyFailedLabel = "Copy failed",
  density = "default",
  themeMode = "auto",
  tone = "default",
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
      data-tone={tone}
      className={cn(
        "relative overflow-hidden border border-line",
        density === "compact" ? "rounded-sm bg-canvas" : "rounded-lg bg-rail",
        codeBlockToneClass(tone),
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
          density === "compact"
            ? "px-3 py-2 text-small-body leading-normal"
            : "px-5 py-4 text-card-title leading-relaxed",
          wrapLines ? "whitespace-pre-wrap break-words" : "whitespace-pre",
          codeBlockToneTextClass(tone),
          label ? (density === "compact" ? "pt-7" : "pt-9") : null,
          copyable ? (density === "compact" ? "pr-10" : "pr-12") : null,
          clampedLines
            ? density === "compact"
              ? "max-h-[calc(var(--code-block-lines)*1.4em+1.5rem)] overflow-y-auto"
              : "max-h-[calc(var(--code-block-lines)*1.6em+2rem)] overflow-y-auto"
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
                  "block min-h-[1.6em]",
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
  copyFailedLabel = "Copy failed",
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
    if (typeof navigator === "undefined" || !navigator.clipboard?.writeText) {
      setCopyState("failed");
      scheduleReset();
      return;
    }

    try {
      await navigator.clipboard.writeText(value);
      setCopyState("copied");
    } catch {
      setCopyState("failed");
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

function codeBlockToneClass(tone: CodeBlockTone): string {
  switch (tone) {
    case "warning":
      return "ring-1 ring-warning/35 bg-warning-tint";
    case "danger":
      return "ring-1 ring-danger/35 bg-danger-tint";
    case "success":
      return "ring-1 ring-success/35 bg-success-tint";
    case "info":
      return "ring-1 ring-info/35 bg-info-tint";
    case "accent":
      return "ring-1 ring-accent/35 bg-accent-tint";
    default:
      return "";
  }
}

function codeBlockToneTextClass(tone: CodeBlockTone): string {
  switch (tone) {
    case "warning":
      return "text-warning";
    case "danger":
      return "text-danger";
    case "success":
      return "text-success";
    case "info":
      return "text-info";
    case "accent":
      return "text-accent";
    default:
      return "";
  }
}

function shouldRenderPrompt(line: string): boolean {
  if (line.length === 0) return false;
  if (/^\s/.test(line)) return false;
  if (line.startsWith("#")) return false;
  return true;
}

export { CodeBlock, CopyIconButton };
