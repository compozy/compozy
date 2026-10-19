import { useSettingsProviders } from "./use-settings-collections";
import { useSettingsPersonaPage } from "./use-settings-persona-page";
import { useSettingsRuntimeCatalog } from "./use-settings-runtime-catalog";

/**
 * Session defaults as General shows them: the acting profile's persona draft
 * (`[defaults]` agent, provider, model and reasoning) and the provider list
 * with its model catalog. Loading waits on the provider list so an unloaded
 * list never reads as "no providers".
 */
export function useSettingsSessionDefaults() {
  const persona = useSettingsPersonaPage();
  const providers = useSettingsProviders();
  const runtime = useSettingsRuntimeCatalog();
  return {
    persona,
    runtime,
    isLoading: persona.isLoading || providers.isLoading,
    error: persona.error ?? providers.error,
    retry: () => {
      persona.handleRetry();
      void providers.refetch();
    },
  };
}
