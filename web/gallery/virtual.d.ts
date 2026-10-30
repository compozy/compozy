declare module "virtual:compozy-prototypes" {
  export const prototypes: ReadonlyArray<
    import("./node/discovery").PrototypeEntry & {
      load: () => Promise<import("./shell/registry-model").PrototypeModule>;
    }
  >;
  export const problems: readonly import("./node/discovery").Problem[];
  export const specs: readonly import("./node/specs").SpecInfo[];
  export const flows: readonly import("./flow").FlowEntry[];
  export const flowProblems: readonly import("./flow").FlowProblem[];
  export const warnings: readonly import("./flow").CoverageWarning[];
}
