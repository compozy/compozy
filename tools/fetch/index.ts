interface FetchInput {
  url: string;
  method?: string;
  headers?: Record<string, string>;
  body?: string | object;
  timeout?: number;
}

interface FetchOutput {
  status: number;
  statusText: string;
  headers: Record<string, string>;
  body: string;
  success: boolean;
}

interface FetchError {
  error: string;
  code?: string;
}

/**
 * Perform HTTP/HTTPS requests
 * @param input - Input containing the request configuration
 * @returns Object containing the response data or error
 */
export async function fetchTool(input: FetchInput): Promise<FetchOutput | FetchError> {
  // Input validation
  if (!input || typeof input !== "object") {
    return {
      error: "Invalid input: input must be an object",
    };
  }
  if (typeof input.url !== "string" || input.url.trim() === "") {
    return {
      error: "Invalid input: url must be a non-empty string",
    };
  }
  
  const url = input.url.trim();
  const method = (input.method || "GET").toUpperCase();
  const timeout = input.timeout || 30000; // Default 30 seconds
  
  // Validate URL format
  try {
    new URL(url);
  } catch {
    return {
      error: `Invalid URL format: ${url}`,
      code: "INVALID_URL",
    };
  }
  
  // Validate HTTP method
  const validMethods = ["GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"];
  if (!validMethods.includes(method)) {
    return {
      error: `Invalid HTTP method: ${method}`,
      code: "INVALID_METHOD",
    };
  }
  
  // Prepare headers
  const headers: Record<string, string> = {
    ...input.headers,
  };
  
  // Handle body and content-type
  let body: string | undefined;
  if (input.body !== undefined && input.body !== null) {
    if (["GET", "HEAD", "OPTIONS"].includes(method)) {
      return {
        error: `HTTP method ${method} cannot have a request body`,
        code: "INVALID_REQUEST",
      };
    }
    
    if (typeof input.body === "object") {
      // Automatically handle JSON
      body = JSON.stringify(input.body);
      if (!headers["Content-Type"] && !headers["content-type"]) {
        headers["Content-Type"] = "application/json";
      }
    } else if (typeof input.body === "string") {
      body = input.body;
    } else {
      return {
        error: "Invalid input: body must be a string or object",
        code: "INVALID_BODY",
      };
    }
  }
  
  // Create AbortController for timeout
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), timeout);
  
  try {
    const response = await fetch(url, {
      method,
      headers,
      body,
      signal: controller.signal,
    });
    
    clearTimeout(timeoutId);
    
    // Get response headers
    const responseHeaders: Record<string, string> = {};
    response.headers.forEach((value, key) => {
      responseHeaders[key] = value;
    });
    
    // Get response body
    const responseBody = await response.text();
    
    return {
      status: response.status,
      statusText: response.statusText,
      headers: responseHeaders,
      body: responseBody,
      success: response.ok,
    };
  } catch (error: any) {
    clearTimeout(timeoutId);
    
    // Handle specific error types
    if (error.name === "AbortError") {
      return {
        error: `Request timeout after ${timeout}ms`,
        code: "TIMEOUT",
      };
    } else if (error.cause?.code === "ECONNREFUSED") {
      return {
        error: `Connection refused: ${url}`,
        code: "ECONNREFUSED",
      };
    } else if (error.cause?.code === "ENOTFOUND") {
      return {
        error: `Host not found: ${url}`,
        code: "ENOTFOUND",
      };
    } else if (error.cause?.code === "CERT_HAS_EXPIRED") {
      return {
        error: `SSL certificate has expired: ${url}`,
        code: "CERT_ERROR",
      };
    } else if (error.cause?.code === "UNABLE_TO_VERIFY_LEAF_SIGNATURE") {
      return {
        error: `SSL certificate verification failed: ${url}`,
        code: "CERT_ERROR",
      };
    } else if (error.code === "ECONNREFUSED") {
      return {
        error: `Connection refused: ${url}`,
        code: "ConnectionRefused",
      };
    } else if (error.code === "ENOTFOUND") {
      return {
        error: `DNS lookup failed: ${url}`,
        code: "DNSLookupFailed",
      };
    } else if (error.message?.includes("Failed to fetch")) {
      return {
        error: `Network error: Failed to fetch from ${url}`,
        code: "NETWORK_ERROR",
      };
    } else {
      return {
        error: `Request failed: ${error.message || "Unknown error"}`,
        code: error.code || "REQUEST_FAILED",
      };
    }
  }
}

// Default export for Compozy runtime compatibility
export default fetchTool;