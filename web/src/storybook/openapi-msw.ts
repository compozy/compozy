import {
  createOpenApiHttp,
  type PathsFor,
  type RequestBodyFor,
  type ResponseBodyFor,
  type ResponseResolverInfo,
} from "openapi-msw";

import type { paths as compozyPaths } from "@/generated/compozy-openapi";

export const compozyApiMock = createOpenApiHttp<compozyPaths>({ baseUrl: "*" });

export type CompozyApiMockMethod = "delete" | "get" | "patch" | "post" | "put";

interface CompozyApiMockHandlersByMethod {
  delete: typeof compozyApiMock.delete;
  get: typeof compozyApiMock.get;
  patch: typeof compozyApiMock.patch;
  post: typeof compozyApiMock.post;
  put: typeof compozyApiMock.put;
}

export type CompozyApiPathForMethod<Method extends CompozyApiMockMethod> = PathsFor<
  CompozyApiMockHandlersByMethod[Method]
>;

export type CompozyApiRequestBodyForMethod<
  Method extends CompozyApiMockMethod,
  Path extends CompozyApiPathForMethod<Method>,
> = RequestBodyFor<CompozyApiMockHandlersByMethod[Method], Path>;

export type CompozyApiResponseBodyForMethod<
  Method extends CompozyApiMockMethod,
  Path extends CompozyApiPathForMethod<Method>,
> = ResponseBodyFor<CompozyApiMockHandlersByMethod[Method], Path>;

export type CompozyApiResponseHelperFor<
  Method extends CompozyApiMockMethod,
  Path extends CompozyApiPathForMethod<Method>,
> = ResponseResolverInfo<compozyPaths, Path, Method>["response"];

type CompozyApiOperationFor<
  Method extends CompozyApiMockMethod,
  Path extends CompozyApiPathForMethod<Method>,
> = Path extends keyof compozyPaths
  ? Method extends keyof compozyPaths[Path]
    ? NonNullable<compozyPaths[Path][Method]>
    : never
  : never;

export type CompozyApiResponsesFor<
  Method extends CompozyApiMockMethod,
  Path extends CompozyApiPathForMethod<Method>,
> = CompozyApiOperationFor<Method, Path> extends { responses: infer Responses } ? Responses : never;

export type CompozyApiPathForMethodStatus<
  Method extends CompozyApiMockMethod,
  Status extends PropertyKey,
> = {
  [Path in CompozyApiPathForMethod<Method>]: Status extends keyof CompozyApiResponsesFor<
    Method,
    Path
  >
    ? Path
    : never;
}[CompozyApiPathForMethod<Method>];

type CompozyApiContentFor<
  Method extends CompozyApiMockMethod,
  Path extends CompozyApiPathForMethod<Method>,
  Status extends keyof CompozyApiResponsesFor<Method, Path>,
> = CompozyApiResponsesFor<Method, Path>[Status] extends { content: infer Content }
  ? Content
  : never;

type JsonMediaType = `${string}/${string}json`;

export type CompozyApiJsonResponseFor<
  Method extends CompozyApiMockMethod,
  Path extends CompozyApiPathForMethod<Method>,
  Status extends keyof CompozyApiResponsesFor<Method, Path>,
> = CompozyApiContentFor<Method, Path, Status>[keyof CompozyApiContentFor<Method, Path, Status> &
  JsonMediaType];

export type CompozyApiOkJsonResponseFor<
  Method extends CompozyApiMockMethod,
  Path extends CompozyApiPathForMethod<Method>,
> = CompozyApiJsonResponseFor<Method, Path, 200 & keyof CompozyApiResponsesFor<Method, Path>>;
