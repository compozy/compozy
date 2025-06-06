// Echo tool for basic runtime testing
export default function run(input: any): Record<string, any> {
  return {
    echo: input, // Return the input object directly, not as a string
    timestamp: new Date().toISOString(),
    type: typeof input,
    tool_name: "echo-tool",
  };
}

// Alternative export for compatibility
export { run };
