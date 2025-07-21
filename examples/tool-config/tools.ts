/**
 * Example tools demonstrating the config parameter feature.
 *
 * The config parameter is passed as the second argument to tool functions,
 * separate from the runtime input. This allows tools to have static
 * configuration that doesn't change between invocations.
 */

interface ApiCallerConfig {
  base_url: string;
  timeout: number;
  retry_count: number;
  headers: Record<string, string>;
}

interface ApiCallerInput {
  endpoint: string;
  method?: "GET" | "POST" | "PUT" | "DELETE";
  data?: any;
}

export async function api_caller(
  input: ApiCallerInput,
  config?: ApiCallerConfig
): Promise<{ status: number; response: any; headers: any }> {
  // Demonstrate using config values
  const baseUrl = config?.base_url || "http://localhost:3000";
  const timeout = config?.timeout || 10;
  const retryCount = config?.retry_count || 1;
  const headers = config?.headers || {};

  console.log("API Caller Config:", {
    baseUrl,
    timeout,
    retryCount,
    headers,
  });

  // Simulate API call (in real implementation, you'd use fetch with the config)
  const fullUrl = `${baseUrl}${input.endpoint}`;

  // Mock response for demonstration
  return {
    status: 200,
    response: {
      id: 123,
      name: "John Doe",
      email: "john@example.com",
      created_at: new Date().toISOString(),
      config_used: {
        url: fullUrl,
        timeout_seconds: timeout,
        retry_attempts: retryCount,
        custom_headers: headers,
      },
    },
    headers: {
      "content-type": "application/json",
      "x-request-id": "demo-" + Math.random().toString(36).substr(2, 9),
    },
  };
}

interface FormatterConfig {
  format: string;
  indent: number;
  sort_keys: boolean;
  date_format: string;
  number_precision: number;
}

interface FormatterInput {
  data: any;
  override_format?: "json" | "yaml" | "xml" | "csv";
}

export async function formatter(
  input: FormatterInput,
  config?: FormatterConfig
): Promise<{ formatted: string; format_used: string }> {
  // Use config values with defaults
  const format = input.override_format || config?.format || "json";
  const indent = config?.indent || 2;
  const sortKeys = config?.sort_keys ?? false;
  const dateFormat = config?.date_format || "ISO8601";
  const numberPrecision = config?.number_precision || 2;

  console.log("Formatter Config:", {
    format,
    indent,
    sortKeys,
    dateFormat,
    numberPrecision,
  });

  let formatted: string;

  switch (format) {
    case "json":
      // Apply formatting based on config
      if (sortKeys && typeof input.data === "object") {
        const sorted = Object.keys(input.data)
          .sort()
          .reduce((obj, key) => {
            obj[key] = input.data[key];
            return obj;
          }, {} as any);
        formatted = JSON.stringify(sorted, null, indent);
      } else {
        formatted = JSON.stringify(input.data, null, indent);
      }
      break;

    case "yaml":
      // Simplified YAML formatting
      formatted = "# YAML format (simplified)\n";
      formatted += objectToYaml(input.data, 0, indent);
      break;

    default:
      formatted = String(input.data);
  }

  return {
    formatted,
    format_used: format,
  };
}

// Helper function for simple YAML conversion
function objectToYaml(obj: any, depth: number, indent: number): string {
  if (obj === null || obj === undefined) return "null\n";
  if (typeof obj !== "object") return String(obj) + "\n";

  let result = "";
  const prefix = " ".repeat(depth * indent);

  for (const [key, value] of Object.entries(obj)) {
    result += `${prefix}${key}: `;
    if (typeof value === "object" && value !== null) {
      result += "\n" + objectToYaml(value, depth + 1, indent);
    } else {
      result += String(value) + "\n";
    }
  }

  return result;
}
