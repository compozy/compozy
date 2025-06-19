// Tool that returns plain text instead of JSON to test error handling
export default function run(input: any): any {
  const message = input.message || "Hello";
  
  // Intentionally return a plain string instead of an object
  // This should test the runtime's ability to handle non-JSON responses
  return `This is plain text response: ${message}`;
}
