/**
 * Memory Tool - Simple memory operations for conversational agent
 * 
 * This tool provides basic memory operations to store and retrieve
 * user conversation history for the simple conversational agent demo.
 */

interface MemoryInput {
  operation: "read" | "append";
  memory_key?: string;
  message?: string;
}

interface AppendResult {
  appended_message: string;
  timestamp: string;
}

interface MemoryOutput {
  success: boolean;
  operation: string;
  result?: string | AppendResult;
  error?: string;
}

export function memoryTool(input: MemoryInput): MemoryOutput {
  // Input validation
  if (!input || typeof input !== 'object') {
    throw new Error('Invalid input: input must be an object');
  }
  
  if (!input.operation || typeof input.operation !== 'string') {
    throw new Error('Invalid input: operation must be a non-empty string');
  }
  
  if (!['read', 'append'].includes(input.operation)) {
    throw new Error('Invalid input: operation must be either "read" or "append"');
  }

  const { operation, memory_key, message } = input;

  try {
    switch (operation) {
      case "read":
        return handleRead(memory_key);
      case "append":
        return handleAppend(memory_key, message);
      default:
        return {
          success: false,
          operation,
          error: `Unknown operation: ${operation}. Supported: read, append`
        };
    }
  } catch (error) {
    return {
      success: false,
      operation,
      error: error instanceof Error ? error.message : "Unknown error occurred"
    };
  }
}

function handleRead(memory_key?: string): MemoryOutput {
  if (!memory_key) {
    return {
      success: false,
      operation: "read",
      error: "memory_key is required for read operation"
    };
  }

  // Simulate reading from memory
  // In real implementation, this would call the memory system
  const mockMemoryContent = simulateUserMemory(memory_key);
  
  return {
    success: true,
    operation: "read",
    result: mockMemoryContent
  };
}

function handleAppend(memory_key?: string, message?: string): MemoryOutput {
  if (!memory_key) {
    return {
      success: false,
      operation: "append",
      error: "memory_key is required for append operation"
    };
  }

  if (!message) {
    return {
      success: false,
      operation: "append",
      error: "message is required for append operation"
    };
  }

  // Simulate appending to memory
  const timestamp = new Date().toISOString();
  const formattedMessage = `[${timestamp}] ${message}`;
  
  return {
    success: true,
    operation: "append",
    result: {
      appended_message: formattedMessage,
      timestamp
    }
  };
}

// Utility function to simulate reading user memory
function simulateUserMemory(memory_key: string): string {
  // Simulate different users' conversation history
  if (memory_key.includes("user:john_doe")) {
    return `User: Hi, I'm John and I live in San Francisco
Agent: Nice to meet you, John! I'll remember that you live in San Francisco.
User: Where do I live?
Agent: You live in San Francisco.`;
  }
  
  if (memory_key.includes("user:jane_smith")) {
    return `User: Hello, my name is Jane and I'm from New York
Agent: Hello Jane! Nice to meet you. I'll remember that you're from New York.`;
  }
  
  // For new users or empty memory
  return "";
}
