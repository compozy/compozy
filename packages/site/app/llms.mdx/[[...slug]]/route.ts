import { type NextRequest, NextResponse } from "next/server";
import { notFound } from "next/navigation";
import { getLLMText } from "@/lib/get-llm-text";
import { docsSource } from "@/lib/source";

export const dynamic = "force-static";
export const revalidate = false;

export function generateStaticParams() {
  return docsSource.generateParams().map(p => ({ slug: ["docs", ...(p.slug ?? [])] }));
}

export async function GET(_req: NextRequest, { params }: { params: Promise<{ slug?: string[] }> }) {
  const { slug = [] } = await params;
  const [tree, ...rest] = slug;
  if (tree !== "docs") notFound();

  const page = docsSource.getPage(rest);
  if (!page) notFound();

  const body = await getLLMText(page);
  return new NextResponse(body, {
    headers: { "Content-Type": "text/markdown; charset=utf-8" },
  });
}
