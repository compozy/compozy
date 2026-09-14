import { Pill, Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from "@compozy/ui";
import { Lock } from "lucide-react";
import Link from "next/link";
import { bindingLabel, INPUT_TYPE_LABELS, type ExtensionInput } from "./marketplace-entry-meta";

/**
 * Declared package inputs, straight from the feed: id, prompt, type, required, binding, and the
 * non-secret default when the package sets one. The guidance names the current install flags
 * (`--input id=value`, `--input-file`); the detail page owns the install command itself.
 */

function defaultLabel(input: ExtensionInput): string | null {
  if (input.default === undefined) return null;
  return typeof input.default === "boolean" ? String(input.default) : input.default;
}

export function MarketplaceEntryInputs({ inputs }: { inputs: readonly ExtensionInput[] }) {
  const hasDefaults = inputs.some(input => input.default !== undefined);
  return (
    <>
      <div className="mt-4 rounded-lg border border-line">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Input</TableHead>
              <TableHead>Prompt</TableHead>
              <TableHead>Type</TableHead>
              <TableHead>Required</TableHead>
              {hasDefaults ? <TableHead>Default</TableHead> : null}
            </TableRow>
          </TableHeader>
          <TableBody>
            {inputs.map(input => {
              const fallback = defaultLabel(input);
              return (
                <TableRow key={input.id}>
                  <TableCell className="align-top whitespace-normal">
                    <code className="font-mono text-fg">{input.id}</code>
                    <span className="mt-0.5 block font-mono text-mono-id text-faint">
                      {bindingLabel(input.binding)}
                    </span>
                  </TableCell>
                  <TableCell className="align-top whitespace-normal text-muted">
                    {input.prompt}
                  </TableCell>
                  <TableCell className="align-top whitespace-normal">
                    {input.type === "secret" ? (
                      <Pill size="sm">
                        <Lock aria-hidden className="size-3" />
                        Secret
                      </Pill>
                    ) : (
                      <span className="text-muted">{INPUT_TYPE_LABELS[input.type]}</span>
                    )}
                  </TableCell>
                  <TableCell className="align-top whitespace-normal text-muted">
                    {input.required ? "Required" : "Optional"}
                  </TableCell>
                  {hasDefaults ? (
                    <TableCell className="align-top whitespace-normal">
                      {fallback !== null ? (
                        <code className="font-mono text-fg">{fallback}</code>
                      ) : (
                        <span className="text-faint">—</span>
                      )}
                    </TableCell>
                  ) : null}
                </TableRow>
              );
            })}
          </TableBody>
        </Table>
      </div>
      <p className="mt-3 text-small-body leading-relaxed text-subtle">
        Pass values at install time with repeatable <code>--input id=value</code> flags; the CLI
        reads the declared type, so a boolean becomes a JSON boolean. Use{" "}
        <code>--input-file inputs.json</code> for structured values or vault references so secrets
        never sit on the command line. The runtime binds each value to the environment variable or
        URL query parameter shown under its id. See{" "}
        <Link
          href="/docs/extensions/install#supply-package-inputs"
          className="underline decoration-line-strong underline-offset-[0.22em] transition-colors hover:text-accent"
        >
          Supply package inputs
        </Link>
        .
      </p>
    </>
  );
}
