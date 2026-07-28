import { type NextRequest, NextResponse } from "next/server";
import { notFound } from "next/navigation";
import { getLLMText } from "@/lib/get-llm-text";
import { protocolDocs, runtimeDocs } from "@/lib/source";

export const dynamic = "force-static";
export const revalidate = false;

type LoaderKey = "runtime" | "protocol";

const loaderByKey: Record<LoaderKey, typeof runtimeDocs> = {
  runtime: runtimeDocs,
  protocol: protocolDocs,
};

export function generateStaticParams() {
  return [
    ...runtimeDocs.generateParams().map(p => ({ slug: ["runtime", ...(p.slug ?? [])] })),
    ...protocolDocs.generateParams().map(p => ({ slug: ["protocol", ...(p.slug ?? [])] })),
  ];
}

export async function GET(_req: NextRequest, { params }: { params: Promise<{ slug?: string[] }> }) {
  const { slug = [] } = await params;
  const [tree, ...rest] = slug;
  const loader = tree === "runtime" || tree === "protocol" ? loaderByKey[tree] : null;
  if (!loader) notFound();

  const page = loader.getPage(rest);
  if (!page) notFound();

  const body = await getLLMText(page);
  return new NextResponse(body, {
    headers: { "Content-Type": "text/markdown; charset=utf-8" },
  });
}
