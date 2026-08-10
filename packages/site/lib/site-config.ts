export const siteConfig = {
  name: "CompozyOS",
  url: "https://compozy.com",
  description:
    "CompozyOS is an agent operating system for real work: it runs agent sessions, keeps their state and memory, applies permissions, connects them to each other, and exposes the whole system through one extensible local-first runtime.",
  githubUrl: "https://github.com/compozy",
  repoUrl: "https://github.com/compozy/compozy",
  repoBranch: "main",
} as const;

export function docsSourceUrl(relativePath: string): string {
  const branch = siteConfig.repoBranch;
  return `${siteConfig.repoUrl}/blob/${branch}/packages/site/content/docs/${relativePath}`;
}

export function absoluteUrl(path = "/") {
  return new URL(path, siteConfig.url).toString();
}

const FILE_EXTENSION_PATTERN = /\.[a-z][a-z0-9]*$/i;

export function canonicalPath(path = "/") {
  if (!path || path === "/") return "/";
  if (path.endsWith("/")) return path;
  if (FILE_EXTENSION_PATTERN.test(path)) return path;
  return `${path}/`;
}

function resolveOGImagePath(path: string): string {
  if (!path || path === "/") return "/opengraph-image";
  const trimmed = path.replace(/^\//, "").replace(/\/$/, "");
  if (!trimmed) return "/opengraph-image";
  const [head] = trimmed.split("/");
  if (head === "docs" || head === "blog") {
    return `/og/${trimmed}/image.png`;
  }
  return "/opengraph-image";
}

export function createPageMetadata({
  title,
  description,
  path,
  image,
  keywords,
}: {
  title: string;
  description?: string;
  path: string;
  image?: {
    url: string;
    alt?: string;
    width?: number;
    height?: number;
  };
  keywords?: readonly string[];
}) {
  const canonical = canonicalPath(path);
  const resolvedDescription = description ?? siteConfig.description;
  const socialImage = {
    url: image?.url ?? resolveOGImagePath(path),
    width: image?.width ?? 1200,
    height: image?.height ?? 630,
    alt: image?.alt ?? `${title} | CompozyOS`,
  };

  return {
    title,
    description: resolvedDescription,
    keywords: keywords && keywords.length > 0 ? Array.from(keywords) : undefined,
    alternates: {
      canonical,
    },
    openGraph: {
      title,
      description: resolvedDescription,
      url: absoluteUrl(canonical),
      siteName: siteConfig.name,
      images: [socialImage],
    },
    twitter: {
      card: "summary_large_image" as const,
      title,
      description: resolvedDescription,
      images: [socialImage.url],
    },
  };
}
