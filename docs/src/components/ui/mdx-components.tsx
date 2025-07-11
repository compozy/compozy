import {
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
  Accordion as BaseAccordion,
} from "@/components/ui/accordion";
import { AccordionGroup, Accordion as AccordionSection } from "@/components/ui/accordion-group";
import { Badge } from "@/components/ui/badge";
import { Button, ButtonArrow } from "@/components/ui/button";
import { Callout } from "@/components/ui/callout";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardHeading,
  CardTable,
  CardTitle,
  CardToolbar,
} from "@/components/ui/card";
import { Code } from "@/components/ui/code";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { CopyButton } from "@/components/ui/copy-button";
import { FeatureCard, FeatureCardList } from "@/components/ui/feature-card";
import { List, ListItem } from "@/components/ui/list";
import { Logo } from "@/components/ui/logo";
import { Mermaid } from "@/components/ui/mermaid";
import { Param, Params, SchemaParams } from "@/components/ui/params";
import { ReferenceCard, ReferenceCardList } from "@/components/ui/reference-card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { Step, Steps } from "@/components/ui/step";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tab, Tabs } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import Link from "fumadocs-core/link";
import { APIPage } from "fumadocs-openapi/ui";
import { TypeTable } from "fumadocs-ui/components/type-table";
import defaultMdxComponents from "fumadocs-ui/mdx";
import type { MDXComponents } from "mdx/types";
import { openapi } from "../../lib/source";

// use this function to get MDX components, you will need it for rendering MDX
export function getMDXComponents(components?: MDXComponents): MDXComponents {
  return {
    ...defaultMdxComponents,
    Link: ({ className, ...props }: React.ComponentProps<typeof Link>) => (
      <Link className={cn("font-medium underline underline-offset-4", className)} {...props} />
    ),
    Accordion: (props: any) => {
      // If it has a title prop, it's meant to be used inside AccordionGroup
      if ("title" in props) {
        return <AccordionSection {...props} />;
      }
      // Otherwise, use the regular Accordion component
      return <BaseAccordion {...props} />;
    },
    AccordionContent,
    AccordionGroup,
    AccordionItem,
    AccordionTrigger,
    Badge,
    Button,
    ButtonArrow,
    Card,
    CardContent,
    CardDescription,
    CardFooter,
    CardHeader,
    CardHeading,
    CardTable,
    CardTitle,
    CardToolbar,
    Collapsible,
    CollapsibleContent,
    CollapsibleTrigger,
    CopyButton,
    FeatureCard,
    FeatureCardList,
    ReferenceCard,
    ReferenceCardList,
    List,
    ListItem,
    Logo,
    Param,
    Params,
    SchemaParams,
    ScrollArea,
    Separator,
    Step,
    Steps,
    Tab,
    Tabs,
    Table,
    TypeTable,
    Callout,
    TableBody,
    TableCaption,
    TableCell,
    TableFooter,
    TableHead,
    TableHeader,
    TableRow,
    Tooltip,
    TooltipContent,
    TooltipProvider,
    TooltipTrigger,
    Code,
    Mermaid,
    pre: (props: any) => (
      <Code {...props} showLineNumbers>
        {props.children}
      </Code>
    ),
    APIPage: (props: any) => <APIPage {...openapi.getAPIPageProps(props)} />,
    ...components,
  };
}
