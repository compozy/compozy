// Simple test tool to verify runtime execution
export async function simple_test(input: {
  message: string;
}): Promise<{ response: string; success: boolean }> {
  console.error("Simple test tool called with:", input);
  return {
    response: `Echo: ${input.message}`,
    success: true,
  };
}
