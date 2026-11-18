/** The SDK overview's lists: the four client pages and every resource with its method count. */
import { cn } from "@compozy/ui";
import { ArrowRight } from "@compozy/ui/icons";
import Link from "next/link";

import { FOCUS, LINE } from "@/components/docs/docs-classes";
import { SdkLangText } from "@/components/sdk/sdk-lang";
import { SDK_GUIDE_PAGES } from "@/lib/sdk-reference-build";
import { sdkReference } from "@/lib/sdk-reference";
import { docsSource } from "@/lib/source";

export function SdkGuideLinks() {
  const guides = SDK_GUIDE_PAGES.flatMap(slug => {
    const page = docsSource.getPage(["sdk", slug]);
    return page ? [page] : [];
  });
  return (
    <ul className={cn("not-prose m-0 list-none divide-y divide-site-light-080 border-y p-0", LINE)}>
      {guides.map(guide => (
        <li key={guide.url}>
          <Link
            href={guide.url}
            className={cn(
              "group grid w-full gap-x-6 gap-y-1 py-4 text-left sm:grid-cols-[140px_minmax(0,1fr)_auto]",
              FOCUS
            )}
          >
            <span className="text-[16px] text-fg">{guide.data.title}</span>
            <span className="text-[14.5px]/[1.55] text-muted-foreground">
              {guide.data.description}
            </span>
            <ArrowRight
              className="size-4 self-center text-subtle transition-transform group-hover:translate-x-1 group-hover:text-fg"
              aria-hidden
            />
          </Link>
        </li>
      ))}
    </ul>
  );
}

export function SdkResourceGrid() {
  const { resources } = sdkReference();
  return (
    <ul className="not-prose m-0 grid list-none gap-3 p-0 sm:grid-cols-2 lg:grid-cols-3">
      {resources.map(resource => (
        <li key={resource.id}>
          <Link
            href={`/docs/sdk/${resource.slug}`}
            className={cn(
              "group flex w-full flex-col gap-3 rounded-[14px] border border-site-light-090 bg-site-light-025 p-4 text-left transition-colors hover:border-site-light-200 hover:bg-site-light-040",
              FOCUS
            )}
          >
            <span className="flex items-baseline justify-between gap-3 text-[15.5px] text-fg">
              {resource.title}
              <span className="font-mono text-[12px] text-subtle tabular-nums">
                {resource.ops.length}
              </span>
            </span>
            <code className="truncate font-mono text-[12.5px] text-muted-foreground">
              <SdkLangText ts={`compozy.${resource.id}`} go="client.Client" />
            </code>
          </Link>
        </li>
      ))}
    </ul>
  );
}
