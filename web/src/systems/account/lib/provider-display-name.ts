import type { SignInProvider } from "@/systems/computers/mocks/remote-computers-fixtures-sign-in";

const PROVIDER_LABEL: Record<SignInProvider, string> = {
  google: "Google",
  github: "GitHub",
};

export function providerDisplayName(provider: SignInProvider): string {
  return PROVIDER_LABEL[provider];
}
