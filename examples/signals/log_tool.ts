interface LogInput {
  message: string;
}

interface LogOutput {
  logged: string;
  timestamp: string;
}

export function logTool(input: LogInput): LogOutput {
  // Input validation
  if (!input || typeof input !== "object") {
    throw new Error("Invalid input: input must be an object");
  }

  if (typeof input.message !== "string") {
    throw new Error("Invalid input: message must be a string");
  }

  // Return success output
  return {
    logged: input.message,
    timestamp: new Date().toISOString(),
  };
}
