import { buttonVariants, cn } from "@compozy/ui";
import { Download } from "lucide-react";
import Link from "next/link";

const DOWNLOAD_PATH = "/download";

/** Global header entry point to the desktop app download page. */
export function DownloadHeaderButton({ className }: { className?: string }) {
  return (
    <Link
      href={DOWNLOAD_PATH}
      className={cn(
        buttonVariants({ variant: "default", size: "default" }),
        "rounded-full",
        className
      )}
    >
      <Download aria-hidden data-icon="inline-start" />
      Download
    </Link>
  );
}
