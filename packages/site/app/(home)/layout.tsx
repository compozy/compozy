import { HomeLayout } from "fumadocs-ui/layouts/home";
import { baseOptions } from "@/lib/layout.shared";
import { HomeHeader } from "@/components/site/home-header";
import { HomeMainContainer } from "@/components/site/home-main-container";
import type { ReactNode } from "react";

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <HomeLayout
      {...baseOptions}
      slots={{
        ...baseOptions.slots,
        header: HomeHeader,
        container: HomeMainContainer,
      }}
    >
      {children}
    </HomeLayout>
  );
}
