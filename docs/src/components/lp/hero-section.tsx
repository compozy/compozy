import { DotPattern } from "@/components/magicui/dot-pattern";
import { AnimatedGroup, type AnimatedGroupProps } from "@/components/ui/animated-group";
import { Button } from "@/components/ui/button";
import { GitHubLogoIcon } from "@radix-ui/react-icons";
import Link from "next/link";

import HeroText from "./hero-text";
const transitionVariants: AnimatedGroupProps["variants"] = {
  item: {
    hidden: {
      opacity: 0,
      filter: "blur(12px)",
      y: 12,
    },
    visible: {
      opacity: 1,
      filter: "blur(0px)",
      y: 0,
      transition: {
        type: "spring",
        bounce: 0.3,
        duration: 1.5,
      },
    },
  },
};

export default function HeroSection() {
  return (
    <>
      <main className="overflow-hidden">
        <div
          aria-hidden
          className="absolute inset-0 isolate hidden opacity-65 contain-strict lg:block"
        >
          <div className="w-140 h-320 -translate-y-87.5 absolute left-0 top-0 -rotate-45 rounded-full bg-[radial-gradient(68.54%_68.72%_at_55.02%_31.46%,hsla(0,0%,85%,.08)_0,hsla(0,0%,55%,.02)_50%,hsla(0,0%,45%,0)_80%)]" />
          <div className="h-320 absolute left-0 top-0 w-60 -rotate-45 rounded-full bg-[radial-gradient(50%_50%_at_50%_50%,hsla(0,0%,85%,.06)_0,hsla(0,0%,45%,.02)_80%,transparent_100%)] [translate:5%_-50%]" />
          <div className="h-320 -translate-y-87.5 absolute left-0 top-0 w-60 -rotate-45 bg-[radial-gradient(50%_50%_at_50%_50%,hsla(0,0%,85%,.04)_0,hsla(0,0%,45%,.02)_80%,transparent_100%)]" />
        </div>
        <section className="relative h-[calc(100vh-4rem)]">
          <div className="absolute inset-0 -z-10 size-full [background:radial-gradient(125%_125%_at_50%_100%,transparent_0%,var(--color-background)_75%)]"></div>
          <DotPattern
            width={20}
            height={20}
            cx={1}
            cy={1}
            cr={1}
            className="absolute inset-0 -z-10 opacity-20 text-neutral-400"
          />
          <div className="absolute inset-0 -z-10 [background:radial-gradient(circle_at_center,transparent_0%,transparent_40%,var(--color-background)_100%)]"></div>
          <div className="relative h-full flex items-center justify-center">
            <div className="mx-auto max-w-7xl px-6">
              <div className="text-center sm:mx-auto lg:mr-auto lg:mt-0 -translate-y-4 sm:-translate-y-8">
                <AnimatedGroup variants={transitionVariants} className="flex justify-center">
                  <a
                    href="https://www.producthunt.com/products/compozy?embed=true&utm_source=badge-featured&utm_medium=badge&utm_source=badge-compozy"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-block"
                  >
                    <img
                      src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=1004723&theme=light&t=1755065549890"
                      alt="Compozy - Design complex AI workflows using intuitive YAML templates | Product Hunt"
                      style={{ width: "250px", height: "54px" }}
                      width="250"
                      height="54"
                      className="mx-auto"
                    />
                  </a>
                </AnimatedGroup>
                <HeroText />
                <AnimatedGroup
                  variants={{
                    container: {
                      visible: {
                        transition: {
                          staggerChildren: 0.05,
                          delayChildren: 0.75,
                        },
                      },
                    },
                    ...transitionVariants,
                  }}
                  className="mt-8 sm:mt-12 flex flex-col items-center justify-center gap-3 sm:flex-row sm:gap-2"
                >
                  <Button asChild size="lg" className="rounded-full">
                    <Link href="/docs/core/getting-started/quick-start">
                      <span className="text-nowrap">Get Started →</span>
                    </Link>
                  </Button>
                  <Button
                    key={2}
                    asChild
                    size="lg"
                    mode="link"
                    variant="ghost"
                    className="h-10.5 rounded-xl px-5 text-foreground/60"
                  >
                    <Link href="https://github.com/compozy/compozy">
                      <GitHubLogoIcon className="size-4" />
                      <span className="text-nowrap">View on GitHub</span>
                    </Link>
                  </Button>
                </AnimatedGroup>
              </div>
            </div>
          </div>
        </section>
      </main>
    </>
  );
}
