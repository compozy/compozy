interface EchoInput {
  message: string;
}

interface EchoOutput {
  echoed_message: string;
  timestamp: string;
}

export function echoTool(input: EchoInput): EchoOutput {
  // Input validation
  if (!input || typeof input !== "object") {
    throw new Error("Invalid input: input must be an object");
  }

  if (typeof input.message !== "string") {
    throw new Error("Invalid input: message must be a string");
  }

  return {
    echoed_message: input.message,
    timestamp: new Date().toISOString(),
  };
}
