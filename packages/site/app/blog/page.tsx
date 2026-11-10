import { BlogPage } from "@/components/blog/blog-page";
import { blogMetadata } from "./metadata";

export const metadata = blogMetadata;

export default function BlogIndexPage() {
  return <BlogPage category={null} />;
}
