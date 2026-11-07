// Vite asset-URL imports used by stories (sprite served as a hashed asset).
declare module "*.svg?url" {
  const url: string;
  export default url;
}

// The face library's keyframes: a side-effect import with no typed shape.
declare module "blobatar/motion.css";

// Vite worker constructor imports (`@compozy/ui/lib/code-highlight-worker`).
declare module "*?worker" {
  const WorkerConstructor: new () => Worker;
  export default WorkerConstructor;
}
