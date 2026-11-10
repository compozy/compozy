import { pageBySlug } from "./chapters";

/** Route head for a chapter: title and description as the v1.2.0 pages set them. */
export function chapterHead(slug: string, description: string) {
  const page = pageBySlug(slug);
  return {
    meta: [
      { title: slug ? `${page.title} · Compozy brand` : "Compozy brand manual" },
      { name: "description", content: description },
    ],
  };
}
