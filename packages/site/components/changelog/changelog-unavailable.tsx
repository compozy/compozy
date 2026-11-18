/** The honest fallback when GitHub release data cannot be loaded. */
import { Empty, cn } from "@compozy/ui";
import { CloudOff } from "@compozy/ui/icons";

import { CONTAINER } from "@/components/site/layout";
import { H1, Muted } from "@/components/site/primitives";
import { COMPOZY_REPOSITORY } from "@/lib/changelog/types";

export function ChangelogUnavailable() {
  return (
    <div className={cn(CONTAINER, "pt-32 pb-28 md:pt-40")}>
      <div className="[&_h1]:md:text-[80px]">
        <H1>Changelog</H1>
      </div>
      <div className="mt-12">
        <Empty
          framed
          icon={<CloudOff />}
          title="The release notes couldn't be loaded"
          description="They come from GitHub, which isn't responding right now."
        />
      </div>
      <Muted className="mt-6">
        You can read them on{" "}
        <a
          href={`${COMPOZY_REPOSITORY.url}/releases`}
          className="text-fg underline underline-offset-4"
        >
          GitHub Releases
        </a>
        .
      </Muted>
    </div>
  );
}
