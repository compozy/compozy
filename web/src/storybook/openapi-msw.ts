import {
  createOpenApiHttp,
  type PathsFor,
  type RequestBodyFor,
  type ResponseBodyFor,
  type ResponseResolverInfo,
} from "openapi-msw";

import type { paths as aghPaths } from "@/generated/agh-openapi";

export const aghApiMock = createOpenApiHttp<aghPaths>({ baseUrl: "*" });

export type AghApiMockMethod = "delete" | "get" | "patch" | "post" | "put";

interface AghApiMockHandlersByMethod {
  delete: typeof aghApiMock.delete;
  get: typeof aghApiMock.get;
  patch: typeof aghApiMock.patch;
  post: typeof aghApiMock.post;
  put: typeof aghApiMock.put;
}

export type AghApiPathForMethod<Method extends AghApiMockMethod> = PathsFor<
  AghApiMockHandlersByMethod[Method]
>;

export type AghApiRequestBodyForMethod<
  Method extends AghApiMockMethod,
  Path extends AghApiPathForMethod<Method>,
> = RequestBodyFor<AghApiMockHandlersByMethod[Method], Path>;

export type AghApiResponseBodyForMethod<
  Method extends AghApiMockMethod,
  Path extends AghApiPathForMethod<Method>,
> = ResponseBodyFor<AghApiMockHandlersByMethod[Method], Path>;

export type AghApiResponseHelperFor<
  Method extends AghApiMockMethod,
  Path extends AghApiPathForMethod<Method>,
> = ResponseResolverInfo<aghPaths, Path, Method>["response"];

type AghApiOperationFor<
  Method extends AghApiMockMethod,
  Path extends AghApiPathForMethod<Method>,
> = Path extends keyof aghPaths
  ? Method extends keyof aghPaths[Path]
    ? NonNullable<aghPaths[Path][Method]>
    : never
  : never;

export type AghApiResponsesFor<
  Method extends AghApiMockMethod,
  Path extends AghApiPathForMethod<Method>,
> = AghApiOperationFor<Method, Path> extends { responses: infer Responses } ? Responses : never;

export type AghApiPathForMethodStatus<
  Method extends AghApiMockMethod,
  Status extends PropertyKey,
> = {
  [Path in AghApiPathForMethod<Method>]: Status extends keyof AghApiResponsesFor<Method, Path>
    ? Path
    : never;
}[AghApiPathForMethod<Method>];

type AghApiContentFor<
  Method extends AghApiMockMethod,
  Path extends AghApiPathForMethod<Method>,
  Status extends keyof AghApiResponsesFor<Method, Path>,
> = AghApiResponsesFor<Method, Path>[Status] extends { content: infer Content } ? Content : never;

type JsonMediaType = `${string}/${string}json`;

export type AghApiJsonResponseFor<
  Method extends AghApiMockMethod,
  Path extends AghApiPathForMethod<Method>,
  Status extends keyof AghApiResponsesFor<Method, Path>,
> = AghApiContentFor<Method, Path, Status>[keyof AghApiContentFor<Method, Path, Status> &
  JsonMediaType];

export type AghApiOkJsonResponseFor<
  Method extends AghApiMockMethod,
  Path extends AghApiPathForMethod<Method>,
> = AghApiJsonResponseFor<Method, Path, 200 & keyof AghApiResponsesFor<Method, Path>>;
