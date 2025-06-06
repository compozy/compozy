// Test tool for Deno runtime testing
export default async function run(input: any): Promise<Record<string, any>> {
  // Simulate some processing delay if requested
  if (input.delay && input.delay > 0) {
    await new Promise(resolve => setTimeout(resolve, input.delay));
  }

  const message = input.message || "Hello from test tool";
  const count = input.count || 1;

  // Generate result based on input
  let result = message;
  if (count > 1) {
    result = Array(count).fill(message).join(" ");
  }

  // Add some environment information for testing
  const environment = Deno.env.get("TEST_ENV") || "unknown";

  // Return structure matching test expectations
  return {
    result: result,
    processed_at: new Date().toISOString(),
    metadata: {
      environment: environment,
      tool_name: "test-tool",
      version: "1.0.0"
    }
  };
}
