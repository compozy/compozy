// Types
export type {
  AllModelsListResponse,
  AllModelsQuery,
  AllModelsRefreshInput,
  AllModelsRefreshRequest,
  AllModelsRefreshResponse,
  ModelAvailabilityState,
  ProviderModelPayload,
  ProviderModelSource,
  ProviderModelSourceStatus,
  ProviderModelsListResponse,
  ProviderModelStatusResponse,
  ProviderModelsQuery,
  ProviderModelsRefreshInput,
  ProviderModelsRefreshRequest,
  ProviderModelsRefreshResponse,
} from "./types";
export { isKnownAvailabilityState, MODEL_AVAILABILITY_STATES } from "./types";

// Adapters
export {
  ModelCatalogApiError,
  getProviderModelStatus,
  listAllModels,
  listProviderModels,
  refreshAllModels,
  refreshProviderModels,
} from "./adapters/model-catalog-api";

// Query infrastructure
export { modelCatalogKeys } from "./lib/query-keys";
export {
  allModelsListOptions,
  providerModelStatusOptions,
  providerModelsListOptions,
  type AllModelsListOptionsArgs,
  type ProviderModelStatusOptionsArgs,
  type ProviderModelsListOptionsArgs,
} from "./lib/query-options";

// Lib
export {
  deriveActiveSessionOptions,
  type ActiveSessionDerivedOptions,
  type DeriveOptionsInput,
  type ModelOption,
  type ReasoningOption,
} from "./lib/derive-active-session-options";
export {
  modelAvailabilityLabel,
  modelAvailabilityTone,
  modelRefreshStateTone,
  providerHealthTone,
  providerStateTone,
} from "./lib/model-catalog-tones";
export {
  providerNeedsAuth,
  toRuntimeModelOptions,
  type ToRuntimeModelOptionsInput,
} from "./lib/to-runtime-selector-options";

// Hooks
export { useProviderModelStatus, useProviderModels } from "./hooks/use-provider-models";
export { useRefreshProviderModels } from "./hooks/use-refresh-provider-models";
export { useRefreshAllModels } from "./hooks/use-refresh-all-models";
export {
  useRuntimeModelCatalog,
  type RuntimeCatalogProvider,
  type RuntimeModelCatalog,
} from "./hooks/use-runtime-model-catalog";
