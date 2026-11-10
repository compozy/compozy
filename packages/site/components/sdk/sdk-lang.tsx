"use client";

/** Content that belongs to one language of the SDK reference. */
import type { ReactNode } from "react";

import { useSdkLang } from "@/components/sdk/hooks/use-sdk-lang";
import type { SdkLang as Lang } from "@/lib/sdk-reference-model";

export function SdkLang({ lang, children }: { lang: Lang; children: ReactNode }) {
  return useSdkLang() === lang ? children : null;
}

/** A short text that reads differently per language, such as a resource's accessor. */
export function SdkLangText({ ts, go }: Record<Lang, string>) {
  return useSdkLang() === "go" ? go : ts;
}
