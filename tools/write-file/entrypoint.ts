// Example entrypoint for testing the write-file tool with Compozy
import writeFile from "./index";

// Export tools with snake_case keys for Compozy runtime
export default {
  write_file: writeFile,
};
