const catalogIcons = import.meta.glob<string>("../../../../../catalog/icons/*.{png,svg,webp}", {
  eager: true,
  import: "default",
  query: "?url",
});

const publishedIconRoot = "https://raw.githubusercontent.com/compozy/compozy/main/catalog/icons/";
const bundledIcons = new Map(
  Object.entries(catalogIcons).map(([path, url]) => [
    publishedIconRoot + path.slice(path.lastIndexOf("/") + 1),
    url,
  ])
);

/** Resolve shipped assets without granting feed URLs access to the browser's network. */
export function marketplaceIconURL(value: string | null | undefined): string | undefined {
  const icon = value?.trim();
  if (!icon) return undefined;
  if (/^data:image\/(?:png|svg\+xml|webp)(?:;base64)?,/.test(icon)) return icon;
  return bundledIcons.get(icon);
}
