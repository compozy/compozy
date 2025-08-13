"use client";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";
import { Building2, CircleCheck, Cloud, Github } from "lucide-react";
import Link from "next/link";
import { tv } from "tailwind-variants";

const pricingStyles = tv({
  slots: {
    section: "py-16 md:py-24 lg:py-32",
    container: "container mx-auto px-4",
    header: "text-center mb-12 md:mb-24",
    headerBadge: "mb-4",
    headerTitle: "!text-foreground text-3xl md:text-4xl lg:text-5xl mb-4",
    headerDescription: "text-muted-foreground text-lg max-w-3xl mx-auto",
    grid: "max-w-6xl mx-auto grid grid-cols-1 lg:grid-cols-3 gap-4 md:gap-6",
    card: "relative bg-background border rounded-2xl p-8 transition-all lg:rounded-none lg:first:rounded-l-2xl lg:last:rounded-r-2xl flex flex-col h-full",
    popularBadge: "absolute -top-3 left-1/2 -translate-x-1/2 px-4",
    cardContent: "mb-6",
    iconWrapper: "inline-flex p-3 rounded-xl bg-muted mb-4",
    icon: "h-6 w-6 text-primary",
    cardTitle: "text-2xl font-medium leading-tight",
    cardPriceWrapper: "flex items-baseline gap-1 mb-4",
    cardPrice: "font-display text-3xl font-semibold",
    cardDescription: "text-muted-foreground",
    separator: "mb-6",
    featureList: "space-y-3 mb-8 flex-grow",
    featureItem: "flex items-start gap-3",
    featureIcon: "h-5 w-5 text-green-500 shrink-0 mt-0.5",
    featureText: "text-sm",
    button: "w-full",
  },
  variants: {
    isPopular: {
      true: {
        card: "!rounded-2xl border-2 border-primary shadow-lg md:scale-105 lg:scale-110 z-10",
      },
    },
    isDisabled: {
      true: {
        card: "",
      },
    },
  },
});

interface Plan {
  name: string;
  price: string | number;
  description: string;
  features: string[];
  buttonText: string;
  buttonVariant: "primary" | "outline" | "secondary" | "ghost" | "destructive";
  buttonHref?: string;
  href?: string;
  icon: typeof Github;
  isPopular?: boolean;
  isDisabled?: boolean;
  isRecommended?: boolean;
}

const plans: Plan[] = [
  {
    name: "Open Source",
    price: "Free",
    description:
      "Full-featured workflow orchestration for individuals and teams building AI applications.",
    features: [
      "Complete workflow engine",
      "All task types (basic, parallel, collection, router)",
      "AI agent management",
      "Custom JS/TS tools with Bun runtime",
      "MCP server integration",
      "Self-hosted deployment",
      "Community support",
    ],
    buttonText: "Get Started",
    buttonVariant: "outline" as const,
    href: "/docs/core/getting-started/installation",
    icon: Github,
  },
  {
    name: "Cloud",
    price: "Coming Soon",
    description:
      "Managed cloud platform with enhanced features for growing teams and production workloads.",
    features: [
      "Everything in Open Source",
      "Managed infrastructure",
      "Automatic scaling",
      "Enhanced monitoring & observability",
      "Priority support",
      "99.9% uptime SLA",
      "Team collaboration features",
    ],
    buttonText: "Deploy",
    buttonVariant: "secondary" as const,
    isPopular: true,
    isDisabled: true,
    icon: Cloud,
  },
  {
    name: "Enterprise",
    price: "Custom",
    description:
      "Advanced features, dedicated support, and custom solutions for mission-critical workloads.",
    features: [
      "Everything in Cloud",
      "On-premise deployment options",
      "Advanced security & compliance",
      "Custom integrations",
      "Dedicated support team",
      "SLA guarantees",
      "Training & onboarding",
    ],
    buttonText: "Contact Sales",
    buttonVariant: "primary" as const,
    buttonHref: "mailto:sales@compozy.com",
    icon: Building2,
  },
] as const;

export const Pricing = () => {
  const styles = pricingStyles();

  const handleExternalButtonClick = (plan: Plan) => {
    if (plan.buttonHref) {
      window.open(plan.buttonHref, plan.name === "Enterprise" ? "_self" : "_blank");
    }
  };

  return (
    <section id="pricing" className={styles.section()}>
      <div className={styles.container()}>
        <div className={styles.header()}>
          <Badge variant="secondary" className={styles.headerBadge()}>
            Simple, Transparent Pricing
          </Badge>
          <h2 className={styles.headerTitle()}>Choose Your Deployment</h2>
          <p className={styles.headerDescription()}>
            Start with our open source by engine, you can self-host Compozy whatever you want. If
            you prefer, soon you will be able to scale with our managed cloud platform.
          </p>
        </div>

        <div className={styles.grid()}>
          {plans.map(plan => {
            const Icon = plan.icon;
            return (
              <div
                key={plan.name}
                className={cn(
                  styles.card({
                    isPopular: plan.isPopular,
                    isDisabled: plan.isDisabled,
                  })
                )}
              >
                {plan.isPopular && <Badge className={styles.popularBadge()}>Coming Soon</Badge>}

                <div className={styles.cardContent()}>
                  <div className={styles.iconWrapper()}>
                    <Icon className={styles.icon()} />
                  </div>
                  <h3 className={styles.cardTitle()}>
                    <span className="text-gradient-gray">{plan.name}</span>
                  </h3>
                  <div className={styles.cardPriceWrapper()}>
                    {typeof plan.price === "string" ? (
                      <span className={`${styles.cardPrice()} text-gradient-green`}>
                        {plan.price}
                      </span>
                    ) : (
                      <>
                        <span className={styles.cardPrice()}>${plan.price}</span>
                        <span className="text-muted-foreground">/month</span>
                      </>
                    )}
                  </div>
                  <p className={styles.cardDescription()}>{plan.description}</p>
                </div>

                <Separator className={styles.separator()} />

                <ul className={styles.featureList()}>
                  {plan.features.map((feature, featureIndex) => (
                    <li key={featureIndex} className={styles.featureItem()}>
                      <CircleCheck className={styles.featureIcon()} />
                      <span className={styles.featureText()}>{feature}</span>
                    </li>
                  ))}
                </ul>

                {plan.href ? (
                  <Button
                    variant={plan.buttonVariant}
                    size="lg"
                    className={styles.button()}
                    disabled={plan.isDisabled}
                    asChild
                  >
                    <Link href={plan.href}>{plan.buttonText}</Link>
                  </Button>
                ) : (
                  <Button
                    variant={plan.buttonVariant}
                    size="lg"
                    className={styles.button()}
                    disabled={plan.isDisabled}
                    onClick={() => handleExternalButtonClick(plan)}
                  >
                    {plan.buttonText}
                  </Button>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
};
