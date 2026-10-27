import type { ReactNode } from "react";

import { Eyebrow } from "@compozy/ui";

interface ContractRowProps {
  label: string;
  children: ReactNode;
}

export function ContractRow({ label, children }: ContractRowProps) {
  return (
    <div className="border-t border-line-soft px-4 py-3 first:border-t-0">
      <Eyebrow className="mb-1 text-faint">{label}</Eyebrow>
      {children}
    </div>
  );
}
