"use client";

import { Button } from "@compozy/ui";
import { Check, Link2 } from "@compozy/ui/icons";
import { useState } from "react";

import { absoluteUrl } from "@/lib/site-config";

export function CopyLink({ permalink }: { permalink: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <Button
      variant="outline"
      onClick={() => {
        // Success shows only once the browser confirms the write; a blocked clipboard stays "Copy link".
        void navigator.clipboard?.writeText(absoluteUrl(`${permalink}/`)).then(
          () => setCopied(true),
          () => setCopied(false)
        );
      }}
    >
      {copied ? <Check /> : <Link2 />}
      {copied ? "Link copied" : "Copy link"}
    </Button>
  );
}
