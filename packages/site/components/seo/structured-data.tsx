import { absoluteUrl, siteConfig } from "@/lib/site-config";

interface JsonLdScriptProps {
  data: Record<string, unknown>;
}

const jsonLdScriptEscapePattern = /[<>&\u2028\u2029]/g;
const WEBSITE_JSON_LD_DATA: Record<string, unknown> = {
  "@context": "https://schema.org",
  "@type": "WebSite",
  name: siteConfig.name,
  description: siteConfig.description,
  url: siteConfig.url,
  inLanguage: "en",
  publisher: {
    "@type": "Organization",
    name: siteConfig.name,
    url: siteConfig.url,
  },
};

function escapeJsonLdScriptCharacter(character: string): string {
  switch (character) {
    case "<":
      return "\\u003c";
    case ">":
      return "\\u003e";
    case "&":
      return "\\u0026";
    case "\u2028":
      return "\\u2028";
    case "\u2029":
      return "\\u2029";
    default:
      return character;
  }
}

function serializeJsonLd(data: Record<string, unknown>): string {
  return JSON.stringify(data).replace(jsonLdScriptEscapePattern, escapeJsonLdScriptCharacter);
}

function JsonLdScript({ data }: JsonLdScriptProps) {
  return <script type="application/ld+json">{serializeJsonLd(data)}</script>;
}

export interface BreadcrumbItem {
  name: string;
  path: string;
}

export function BreadcrumbListJsonLd({ items }: { items: BreadcrumbItem[] }) {
  if (items.length === 0) return null;
  const data = {
    "@context": "https://schema.org",
    "@type": "BreadcrumbList",
    itemListElement: items.map((item, index) => ({
      "@type": "ListItem",
      position: index + 1,
      name: item.name,
      item: absoluteUrl(item.path),
    })),
  };
  return <JsonLdScript data={data} />;
}

export interface TechArticleJsonLdProps {
  title: string;
  description?: string;
  path: string;
  imageUrl?: string;
}

export function TechArticleJsonLd({ title, description, path, imageUrl }: TechArticleJsonLdProps) {
  const data: Record<string, unknown> = {
    "@context": "https://schema.org",
    "@type": "TechArticle",
    headline: title,
    name: title,
    url: absoluteUrl(path),
    inLanguage: "en",
    isPartOf: {
      "@type": "WebSite",
      name: siteConfig.name,
      url: siteConfig.url,
    },
    publisher: {
      "@type": "Organization",
      name: siteConfig.name,
      url: siteConfig.url,
    },
  };
  if (description) data.description = description;
  if (imageUrl) data.image = imageUrl.startsWith("http") ? imageUrl : absoluteUrl(imageUrl);
  return <JsonLdScript data={data} />;
}

export interface ArticleJsonLdProps {
  title: string;
  description?: string;
  path: string;
  imageUrl?: string;
  datePublished: string;
  dateModified?: string;
  authorName?: string;
  keywords?: readonly string[];
}

export function ArticleJsonLd({
  title,
  description,
  path,
  imageUrl,
  datePublished,
  dateModified,
  authorName,
  keywords,
}: ArticleJsonLdProps) {
  const data: Record<string, unknown> = {
    "@context": "https://schema.org",
    "@type": "Article",
    headline: title,
    name: title,
    url: absoluteUrl(path),
    inLanguage: "en",
    datePublished,
    dateModified: dateModified ?? datePublished,
    mainEntityOfPage: {
      "@type": "WebPage",
      "@id": absoluteUrl(path),
    },
    publisher: {
      "@type": "Organization",
      name: siteConfig.name,
      url: siteConfig.url,
    },
  };
  if (description) data.description = description;
  if (imageUrl) data.image = imageUrl.startsWith("http") ? imageUrl : absoluteUrl(imageUrl);
  if (authorName) {
    data.author = {
      "@type": "Person",
      name: authorName,
    };
  }
  if (keywords && keywords.length > 0) data.keywords = keywords.join(", ");
  return <JsonLdScript data={data} />;
}

export function WebSiteJsonLd() {
  return <JsonLdScript data={WEBSITE_JSON_LD_DATA} />;
}
