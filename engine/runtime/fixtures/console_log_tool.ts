// Tool that uses console.log to test stdout pollution fix
export default function run(input: any): Record<string, any> {
  const message = input.message || "default message";
  const logCount = input.log_count || 1;
  
  // These console.log calls should now go to stderr instead of stdout
  for (let i = 0; i < logCount; i++) {
    console.log(`Debug log ${i + 1}: Processing ${message}`);
  }
  
  // Also test other console methods
  console.log("This is a regular log");
  console.debug("This is a debug message");
  console.info("This is an info message");
  console.warn("This is a warning");
  
  // Return a proper JSON response
  return {
    success: true,
    message: `Processed: ${message}`,
    logs_written: logCount,
    timestamp: new Date().toISOString(),
  };
}
