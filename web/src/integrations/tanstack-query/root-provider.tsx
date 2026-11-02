import { QueryClientProvider, type QueryClient } from "@tanstack/react-query";

import { AgentIdentityProvider } from "@/systems/agent/components/agent-identity-provider";

export function Provider({
  children,
  queryClient,
}: {
  children: React.ReactNode;
  queryClient: QueryClient;
}) {
  return (
    <QueryClientProvider client={queryClient}>
      <AgentIdentityProvider>{children}</AgentIdentityProvider>
    </QueryClientProvider>
  );
}
