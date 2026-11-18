import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { BundledDetail } from "@/components/marketplace/bundled-detail";
import { bundledExtensions } from "@/lib/marketplace-bundled";
import { bundledExtensionDescription } from "@/lib/marketplace-copy";
import { createPageMetadata } from "@/lib/site-config";
import { docsSource } from "@/lib/source";

interface PageProps {
  params: Promise<{ name: string }>;
}

export function generateStaticParams() {
  return bundledExtensions.map(extension => ({ name: extension.name }));
}

function findExtension(name: string) {
  const extension = bundledExtensions.find(candidate => candidate.name === name);
  if (!extension) notFound();
  return extension;
}

export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const extension = findExtension((await params).name);
  return createPageMetadata({
    title: `${extension.displayName} — Marketplace`,
    description: bundledExtensionDescription(extension.description),
    path: extension.path,
  });
}

export default async function BundledExtensionPage({ params }: PageProps) {
  const extension = findExtension((await params).name);
  const walkthroughs = new Map<string, string>();
  for (const loop of extension.loops) {
    const url = docsSource.getPage(["examples", `${loop.name}-loop`])?.url;
    if (url) walkthroughs.set(loop.name, url);
  }
  return <BundledDetail extension={extension} walkthroughs={walkthroughs} />;
}
