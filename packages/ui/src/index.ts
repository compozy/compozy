// Surface contract for @compozy/ui. The primitive inventory lives in
// `primitives.ts` (core shadcn/base-nova primitives) plus one named-re-export
// file per domain under `exports/` — every symbol a consumer can import is
// spelled out in exactly one of those files.
export * from "./primitives";
export * from "./exports/shell";
export * from "./exports/menus";
export * from "./exports/listing";
export * from "./exports/viz";
export * from "./exports/editors";
export * from "./exports/conversation";
export * from "./exports/foundation";
export * from "./exports/animation";
