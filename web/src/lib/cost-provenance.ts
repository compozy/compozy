import type { OperationResponse } from "@/lib/api-contract";

/**
 * Single owner of the cost money-vs-word decision shared by the session Usage
 * inspector and the task-run detail header. `cost_status` is authoritative;
 * consumers render the returned pieces in their own chrome but never re-derive
 * the amount. The status/source enums are derived from the generated usage
 * contract, so a new daemon-side value fails typecheck here.
 */
type SessionUsage = OperationResponse<"getSessionUsage", 200>["usage"];

export type CostStatus = NonNullable<SessionUsage["cost_status"]>;
export type CostSource = NonNullable<SessionUsage["cost_source"]>;

export interface CostInput {
  status?: CostStatus | null;
  source?: CostSource | null;
  amount?: number | null;
  currency?: string | null;
}

export interface CostDisplay {
  /** Normalized status; `"unknown"` when the daemon reported no cost status. */
  status: CostStatus;
  /**
   * Primary display string, identical on every surface:
   * `"$0.18"` (actual) · `"≈ $0.18"` (estimated) · `"Included"` · `"Unavailable"` · `"—"`.
   */
  value: string;
  /** True only when `value` is a monetary amount (actual or estimated). */
  isAmount: boolean;
  /** True when the amount is a catalog estimate — drives the `≈` glyph and "Estimated" wording. */
  isEstimated: boolean;
  /** Human provenance label for actual/estimated; `null` for included/unknown/none. */
  sourceLabel: string | null;
  /**
   * Secondary provenance line (identical copy on any surface that shows it):
   * source for actual, "Estimated · <source>" for estimated, "Subscription" for
   * included, "Cost unavailable" for unknown, `null` when there is nothing to add.
   */
  note: string | null;
  /** True when the daemon declared a cost status; an absent status renders nothing. */
  hasCost: boolean;
}

const NO_AMOUNT = "—";
const APPROX_PREFIX = "≈ ";

const COST_SOURCE_LABELS: Record<CostSource, string | null> = {
  agent_reported: "Reported by agent",
  catalog_config: "Catalog rate",
  models_dev: "models.dev rate",
  builtin: "Built-in rate",
  none: null,
};

const currencyFormatters = new Map<string, Intl.NumberFormat>();

function currencyFormatter(currency: string, digits: number): Intl.NumberFormat {
  const key = `${currency}:${digits}`;
  const cached = currencyFormatters.get(key);
  if (cached) return cached;
  const formatter = Intl.NumberFormat(undefined, {
    style: "currency",
    currency,
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  });
  currencyFormatters.set(key, formatter);
  return formatter;
}

function normalizedCurrencyCode(currency?: string | null): string {
  const code = currency?.trim().toUpperCase();
  return code && /^[A-Z]{3}$/.test(code) ? code : "USD";
}

/** Formats a finite amount; sub-$1 values keep 3 fraction digits so cents stay legible. */
function formatAmount(amount: number, currency?: string | null): string {
  const digits = Math.abs(amount) < 1 ? 3 : 2;
  return currencyFormatter(normalizedCurrencyCode(currency), digits).format(amount);
}

function resolveSourceLabel(source?: CostSource | null): string | null {
  if (!source) return null;
  return COST_SOURCE_LABELS[source] ?? null;
}

/**
 * Resolves the truthful presentation of a cost record. `included` and `unknown`
 * carry no monetary amount; `estimated` is always marked with the `≈` glyph; an
 * absent status renders nothing even when an amount is present.
 */
export function describeCost(input: CostInput): CostDisplay {
  const rawStatus = input.status ?? undefined;
  const amount =
    typeof input.amount === "number" && Number.isFinite(input.amount) ? input.amount : null;
  const formatted = amount !== null ? formatAmount(amount, input.currency) : null;
  const sourceLabel = resolveSourceLabel(input.source);

  if (rawStatus === "included") {
    return {
      status: "included",
      value: "Included",
      isAmount: false,
      isEstimated: false,
      sourceLabel: null,
      note: "Subscription",
      hasCost: true,
    };
  }

  if (rawStatus === "unknown") {
    return {
      status: "unknown",
      value: "Unavailable",
      isAmount: false,
      isEstimated: false,
      sourceLabel: null,
      note: "Cost unavailable",
      hasCost: true,
    };
  }

  if (rawStatus === "estimated") {
    const isEstimated = formatted !== null;
    const estimatedNote = sourceLabel ? `Estimated · ${sourceLabel}` : "Estimated";
    return {
      status: "estimated",
      value: isEstimated ? `${APPROX_PREFIX}${formatted}` : NO_AMOUNT,
      isAmount: isEstimated,
      isEstimated,
      sourceLabel,
      note: isEstimated ? estimatedNote : sourceLabel,
      hasCost: true,
    };
  }

  if (rawStatus === "actual") {
    return {
      status: "actual",
      value: formatted ?? NO_AMOUNT,
      isAmount: formatted !== null,
      isEstimated: false,
      sourceLabel,
      note: sourceLabel,
      hasCost: true,
    };
  }

  return {
    status: "unknown",
    value: NO_AMOUNT,
    isAmount: false,
    isEstimated: false,
    sourceLabel: null,
    note: null,
    hasCost: false,
  };
}
