import { createFileRoute, redirect } from "@tanstack/react-router";

import { isHostedOriginCandidate } from "@/lib/hosted-browser-auth";
import {
  accountStatusOptions,
  sanitizeSignInRedirect,
  SignInScreen,
  useSignInPage,
} from "@/systems/account";

interface SignInRouteSearch {
  redirect?: string;
}

function SignInRoutePage() {
  const search = Route.useSearch();
  const page = useSignInPage({ redirectTo: search.redirect });

  return (
    <SignInScreen
      autoFocusFirstProvider
      buttonsDisabled={page.view.buttonsDisabled}
      isUnlinking={page.isUnlinking}
      marketingOptIn={page.marketingOptIn}
      notice={page.view.notice}
      onBackToApp={page.handleBackToApp}
      onCancelWaiting={page.handleCancelWaiting}
      onMarketingOptInChange={page.setMarketingOptIn}
      onReopenBrowser={page.handleReopenBrowser}
      onRetryConnection={page.handleRetryConnection}
      onSelectProvider={page.handleSelectProvider}
      onUnlinkInstallation={page.handleUnlinkInstallation}
      subtitle={page.view.subtitle}
      title={page.view.title}
      tokenRevoked={page.view.tokenRevoked}
      waiting={page.waiting}
    />
  );
}

export const Route = createFileRoute("/sign-in")({
  validateSearch: (search: Record<string, unknown>): SignInRouteSearch => ({
    redirect: sanitizeSignInRedirect(search.redirect),
  }),
  beforeLoad: async ({ context, search }) => {
    if (isHostedOriginCandidate()) return;
    const status = await context.queryClient
      .ensureQueryData(accountStatusOptions())
      .catch(() => undefined);
    if (!status) return;
    if (status.gate === "off" || status.signed_in || status.state === "signed_in") {
      throw redirect({ href: sanitizeSignInRedirect(search.redirect) ?? "/" });
    }
  },
  component: SignInRoutePage,
});
