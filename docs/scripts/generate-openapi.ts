import { generateFiles } from "fumadocs-openapi";

// Generate MDX files from OpenAPI/Swagger documentation
void generateFiles({
  // Use the generated swagger.json from the public directory
  input: ["./swagger.json"],
  // Output to the content directory with absolute path
  output: "./content/docs/api",
  // Generate pages grouped by tag
  per: "tag",
  // Include descriptions from OpenAPI spec
  includeDescription: true,
  // Add a comment at the top of generated files
  addGeneratedComment:
    "<!-- This file was auto-generated from OpenAPI/Swagger. Do not edit manually. -->",
  // Custom frontmatter for generated pages
  frontmatter: (title, description) => ({
    title,
    description,
    icon: "Code",
    group: "API Reference",
  }),
});
