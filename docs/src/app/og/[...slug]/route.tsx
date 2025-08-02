import { generateOGImage } from "@/app/og/[...slug]/og";
import { source } from "@/lib/source";
import { notFound } from "next/navigation";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const fontsDir = join(process.cwd(), "src/app/og/[...slug]");
const clashRegular = readFileSync(join(fontsDir, "ClashDisplay-Regular.ttf"));
const clashLight = readFileSync(join(fontsDir, "ClashDisplay-Light.ttf"));
const clashBold = readFileSync(join(fontsDir, "ClashDisplay-Semibold.ttf"));
const geistRegular = readFileSync(join(fontsDir, "Geist-Regular.ttf"));

export async function GET(req: Request, { params }: { params: Promise<{ slug: string[] }> }) {
  const { slug } = await params;
  const page = source.getPage(slug.slice(0, -1));
  if (!page) notFound();

  // Get the origin from the request URL
  const { origin } = new URL(req.url);
  const logoUrl = `${origin}/full_logo.png`;

  // Truncate description to max 165 characters
  const truncatedDescription = page.data.description
    ? page.data.description.length > 165
      ? page.data.description.substring(0, 162) + "..."
      : page.data.description
    : undefined;

  return generateOGImage({
    primaryTextColor: "rgb(240,240,240)",
    title: page.data.title,
    description: truncatedDescription,
    logoUrl,
    fonts: [
      {
        name: "Clash",
        data: clashRegular,
        weight: 400,
      },
      {
        name: "Clash",
        data: clashBold,
        weight: 600,
      },
      {
        name: "Clash",
        data: clashLight,
        weight: 200,
      },
      {
        name: "Geist",
        data: geistRegular,
        weight: 400,
      },
    ],
  });
}

export function generateStaticParams(): {
  slug: string[];
}[] {
  return source.generateParams().map(page => ({
    ...page,
    slug: [...page.slug, "image.png"],
  }));
}
