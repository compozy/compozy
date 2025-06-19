// Tool that throws an error or returns undefined to test error handling
export default function run(input: any): any {
  const mode = input.mode || "undefined";
  
  switch (mode) {
    case "throw":
      throw new Error("Tool execution failed with error");
    case "undefined":
      return undefined;
    case "null":
      return null;
    case "number":
      return 42;
    case "boolean":
      return true;
    case "array":
      return ["item1", "item2", "item3"];
    case "complex":
      return {
        nested: {
          data: "complex object",
          array: [1, 2, 3],
          bool: false
        }
      };
    default:
      return undefined;
  }
}
