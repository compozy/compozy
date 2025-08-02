import type { MetadataRoute } from "next";

export default function robots(): MetadataRoute.Robots {
  const baseUrl = process.env.NODE_ENV === "production" 
    ? "https://compozy.com" 
    : "http://localhost:5006";

  return {
    rules: {
      userAgent: "*",
      allow: "/",
      disallow: ["/api/", "/_next/", "/og/"],
    },
    sitemap: `${baseUrl}/sitemap.xml`,
  };
}
