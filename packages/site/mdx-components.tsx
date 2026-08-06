import defaultMdxComponents from "fumadocs-ui/mdx";
import {
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
    return <OpenAPIPageClient {...props} preloaded={openapiPreload.preloaded} />;
  }

  return OpenAPIPage;
}

export function getMDXComponents(openapiPreload?: OpenAPIPagePreload) {
  const OpenAPIPage = createPreloadedOpenAPIPage(openapiPreload);
  return {
    ...defaultMdxComponents,
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
    Workflow,
    WorkflowStep,
  };
}
