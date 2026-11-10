import defaultMdxComponents from "fumadocs-ui/mdx";
import samples from "../../openapi/sdk-samples.json";
import { withSDKCodeSamples, type SDKSampleManifest } from "@/lib/sdk-samples";
import {
  DocsH2,
  DocsH3,
  DocsInlineCode,
  DocsLink,
  DocsPre,
  DocsTable,
} from "@/components/docs/docs-prose";
import {
  Callout,
  GuideCard,
  GuideGrid,
  OperatorNote,
  RouteList,
  RouteRow,
  Workflow,
  WorkflowStep,
} from "@/components/docs/mdx-blocks";
import { GroupIndex, GroupRow } from "@/components/docs/group-index";
import { Mermaid } from "@/components/docs/mermaid";
import { PathCard, PathGrid, PathLink } from "@/components/docs/path-grid";
import { CurrentReleaseTag, InstallCommand } from "@/components/docs/release-install";
import { SdkLang } from "@/components/sdk/sdk-lang";
import { SdkLangBar } from "@/components/sdk/sdk-lang-bar";
import { SdkGuideLinks, SdkResourceGrid } from "@/components/sdk/sdk-overview";
import { SdkResource } from "@/components/sdk/sdk-resource";
import { SdkFieldRow, SdkFields, SdkSymbol } from "@/components/sdk/sdk-symbol";
import {
  OpenAPIPage as OpenAPIPageClient,
  type OpenAPIPageProps_Preloaded,
} from "@/components/docs/openapi-page";

type OpenAPIPagePreload = Pick<OpenAPIPageProps_Preloaded, "preloaded">;
type GeneratedOpenAPIPageProps = Omit<OpenAPIPageProps_Preloaded, "preloaded">;

function createPreloadedOpenAPIPage(openapiPreload: OpenAPIPagePreload | undefined) {
  function OpenAPIPage(props: GeneratedOpenAPIPageProps) {
    if (!openapiPreload) {
      throw new Error("OpenAPIPage rendered without preloaded OpenAPI documents.");
    }
    return (
      <OpenAPIPageClient
        {...withSDKCodeSamples(
          { ...props, preloaded: openapiPreload.preloaded },
          samples as SDKSampleManifest
        )}
      />
    );
  }

  return OpenAPIPage;
}

export function getMDXComponents(openapiPreload?: OpenAPIPagePreload) {
  const OpenAPIPage = createPreloadedOpenAPIPage(openapiPreload);
  return {
    ...defaultMdxComponents,
    h2: DocsH2,
    h3: DocsH3,
    a: DocsLink,
    code: DocsInlineCode,
    pre: DocsPre,
    table: DocsTable,
    Callout,
    OpenAPIPage,
    APIPage: OpenAPIPage,
    CurrentReleaseTag,
    InstallCommand,
    Mermaid,
    GroupIndex,
    GroupRow,
    GuideCard,
    GuideGrid,
    OperatorNote,
    PathCard,
    PathGrid,
    PathLink,
    RouteList,
    RouteRow,
    SdkField: SdkFieldRow,
    SdkFields,
    SdkGuideLinks,
    SdkLang,
    SdkLangBar,
    SdkResource,
    SdkResourceGrid,
    SdkSymbol,
    Workflow,
    WorkflowStep,
  };
}
