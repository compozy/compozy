import type { RuntimeModelOption } from "@/systems/runtime";

interface SessionModelSelectionInput {
  provider: string;
  model: string;
  models: RuntimeModelOption[];
  catalogLoading: boolean;
  catalogLoaded: boolean;
  catalogError: string | null;
}

interface SessionModelSelectionValidation {
  valid: boolean;
  error: string | null;
}

const MODEL_NOT_IN_CATALOG =
  "This model is not in the selected provider catalog. Choose a listed model or leave Model blank to use the provider default.";

const MODEL_CATALOG_UNAVAILABLE =
  "Model catalog is unavailable. Refresh it or leave Model blank to use the provider default.";

const MODEL_CATALOG_PENDING = "Wait for the model catalog to finish loading.";

// Keep this boundary aligned with modelcatalog.HasAuthoritativeProviderCatalog.
// Other providers may accept custom or dynamically discovered model identifiers.
const AUTHORITATIVE_MODEL_CATALOG_PROVIDERS = new Set(["cursor"]);

function validateSessionModelSelection({
  provider,
  model,
  models,
  catalogLoading,
  catalogLoaded,
  catalogError,
}: SessionModelSelectionInput): SessionModelSelectionValidation {
  const selectedModel = model.trim();
  if (selectedModel.length === 0) {
    return { valid: true, error: null };
  }
  if (!AUTHORITATIVE_MODEL_CATALOG_PROVIDERS.has(provider.trim())) {
    return { valid: true, error: null };
  }
  if (catalogError) {
    return { valid: false, error: MODEL_CATALOG_UNAVAILABLE };
  }
  if (catalogLoading || !catalogLoaded) {
    return { valid: false, error: null };
  }

  const selectedProvider = provider.trim();
  const exactMatch = models.find(
    option => option.provider === selectedProvider && option.id === selectedModel
  );
  if (
    exactMatch === undefined ||
    exactMatch.disabled === true ||
    exactMatch.availability === "unavailable"
  ) {
    return { valid: false, error: MODEL_NOT_IN_CATALOG };
  }
  return { valid: true, error: null };
}

export { MODEL_CATALOG_PENDING, validateSessionModelSelection };
