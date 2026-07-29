function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function nonEmptyString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

/** Preserve the provider's most useful failure text across transcript adapters. */
export function toolPartErrorText(part: Record<string, unknown>): string {
  const direct = nonEmptyString(part.errorText) ?? nonEmptyString(part.error);
  if (direct) return direct;

  const output = part.output;
  if (isRecord(output)) {
    const fromOutput =
      nonEmptyString(output.error) ?? nonEmptyString(output.text) ?? nonEmptyString(output.title);
    if (fromOutput) return fromOutput;
  }
  const outputText = nonEmptyString(output);
  if (outputText) return outputText;

  return "Tool execution failed";
}

/** Convert an output-error part into a renderable result without dropping its output payload. */
export function toolPartResult(part: Record<string, unknown>, isError: boolean): unknown {
  const output = part.output;
  if (!isError) return output;

  const error = toolPartErrorText(part);
  if (isRecord(output)) {
    return { ...output, error };
  }
  if (nonEmptyString(output)) {
    return { content: output, error };
  }
  return { error };
}
