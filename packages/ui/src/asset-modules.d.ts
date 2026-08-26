// Vite asset-URL imports used by stories (sprite served as a hashed asset).
declare module "*.svg?url" {
  const url: string;
  export default url;
}
