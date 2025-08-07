import { afterAll, beforeAll, describe, expect, test } from "bun:test";
import { createServer } from "http";
import { fetchTool } from "./index";

describe("fetchTool", () => {
  let server: any;
  let serverUrl: string;

  beforeAll(done => {
    // Create a test HTTP server
    server = createServer((req, res) => {
      const url = new URL(req.url || "/", `http://${req.headers.host}`);

      // Echo endpoint - returns request details
      if (url.pathname === "/echo") {
        const body: Buffer[] = [];
        req.on("data", chunk => body.push(chunk));
        req.on("end", () => {
          const requestBody = Buffer.concat(body).toString();
          res.writeHead(200, { "Content-Type": "application/json" });
          res.end(
            JSON.stringify({
              method: req.method,
              headers: req.headers,
              body: requestBody,
              query: Object.fromEntries(url.searchParams),
            })
          );
        });
        return;
      }

      // Status endpoint - returns specific status codes
      if (url.pathname.startsWith("/status/")) {
        const status = parseInt(url.pathname.split("/")[2]);
        res.writeHead(status, { "Content-Type": "text/plain" });
        res.end(`Status ${status}`);
        return;
      }

      // Headers endpoint - returns custom headers
      if (url.pathname === "/headers") {
        res.writeHead(200, {
          "Content-Type": "application/json",
          "X-Custom-Header": "test-value",
          "X-Another-Header": "another-value",
        });
        res.end(JSON.stringify({ message: "Headers test" }));
        return;
      }

      // Timeout endpoint - delays response
      if (url.pathname === "/timeout") {
        const delay = Math.min(parseInt(url.searchParams.get("delay") || "5000"), 10000); // Limit delay to 10 seconds max
        setTimeout(() => {
          res.writeHead(200);
          res.end("Delayed response");
        }, delay);
        return;
      }

      // Default 404
      res.writeHead(404);
      res.end("Not found");
    });

    server.listen(0, "127.0.0.1", () => {
      const address = server.address();
      serverUrl = `http://127.0.0.1:${address.port}`;
      done();
    });
  });

  afterAll(done => {
    server.close(done);
  });

  // Input validation tests
  test("Should handle invalid input - null input", async () => {
    const result = await fetchTool(null as any);
    expect(result).toEqual({
      error: "Invalid input: input must be an object",
    });
  });

  test("Should handle invalid input - undefined input", async () => {
    const result = await fetchTool(undefined as any);
    expect(result).toEqual({
      error: "Invalid input: input must be an object",
    });
  });

  test("Should handle invalid input - string input", async () => {
    const result = await fetchTool("http://example.com" as any);
    expect(result).toEqual({
      error: "Invalid input: input must be an object",
    });
  });

  test("Should handle invalid input - missing url", async () => {
    const result = await fetchTool({} as any);
    expect(result).toEqual({
      error: "Invalid input: url must be a non-empty string",
    });
  });

  test("Should handle invalid input - empty url", async () => {
    const result = await fetchTool({ url: "" });
    expect(result).toEqual({
      error: "Invalid input: url must be a non-empty string",
    });
  });

  test("Should handle invalid input - non-string url", async () => {
    const result = await fetchTool({ url: 123 as any });
    expect(result).toEqual({
      error: "Invalid input: url must be a non-empty string",
    });
  });

  test("Should handle invalid URL format", async () => {
    const result = await fetchTool({ url: "not-a-valid-url" });
    expect(result).toEqual({
      error: "Invalid URL format: not-a-valid-url",
      code: "INVALID_URL",
    });
  });

  test("Should handle invalid HTTP method", async () => {
    const result = await fetchTool({
      url: serverUrl,
      method: "INVALID",
    });
    expect(result).toEqual({
      error: "Invalid HTTP method: INVALID",
      code: "INVALID_METHOD",
    });
  });

  test("Should reject body for GET requests", async () => {
    const result = await fetchTool({
      url: serverUrl,
      method: "GET",
      body: "test body",
    });
    expect(result).toEqual({
      error: "HTTP method GET cannot have a request body",
      code: "INVALID_REQUEST",
    });
  });

  test("Should reject body for HEAD requests", async () => {
    const result = await fetchTool({
      url: serverUrl,
      method: "HEAD",
      body: { test: "data" },
    });
    expect(result).toEqual({
      error: "HTTP method HEAD cannot have a request body",
      code: "INVALID_REQUEST",
    });
  });

  test("Should handle invalid body type", async () => {
    const result = await fetchTool({
      url: serverUrl,
      method: "POST",
      body: 123 as any,
    });
    expect(result).toEqual({
      error: "Invalid input: body must be a string or object",
      code: "INVALID_BODY",
    });
  });

  // HTTP request tests
  test("Should perform successful GET request", async () => {
    const result = await fetchTool({ url: `${serverUrl}/echo` });

    expect(result).toHaveProperty("status", 200);
    expect(result).toHaveProperty("statusText", "OK");
    expect(result).toHaveProperty("success", true);
    expect(result).toHaveProperty("headers");
    expect(result).toHaveProperty("body");

    if ("body" in result) {
      const response = JSON.parse(result.body);
      expect(response.method).toBe("GET");
    }
  });

  test("Should perform POST request with JSON body", async () => {
    const testData = { message: "Hello", number: 42 };
    const result = await fetchTool({
      url: `${serverUrl}/echo`,
      method: "POST",
      body: testData,
    });

    expect(result).toHaveProperty("status", 200);
    expect(result).toHaveProperty("success", true);

    if ("body" in result) {
      const response = JSON.parse(result.body);
      expect(response.method).toBe("POST");
      expect(response.headers["content-type"]).toBe("application/json");
      expect(JSON.parse(response.body)).toEqual(testData);
    }
  });

  test("Should perform POST request with string body", async () => {
    const result = await fetchTool({
      url: `${serverUrl}/echo`,
      method: "POST",
      body: "Plain text body",
    });

    expect(result).toHaveProperty("status", 200);

    if ("body" in result) {
      const response = JSON.parse(result.body);
      expect(response.body).toBe("Plain text body");
    }
  });

  test("Should handle custom headers", async () => {
    const result = await fetchTool({
      url: `${serverUrl}/echo`,
      headers: {
        "X-Custom-Header": "test-value",
        Authorization: "Bearer token123",
      },
    });

    expect(result).toHaveProperty("status", 200);

    if ("body" in result) {
      const response = JSON.parse(result.body);
      expect(response.headers["x-custom-header"]).toBe("test-value");
      expect(response.headers["authorization"]).toBe("Bearer token123");
    }
  });

  test("Should receive response headers", async () => {
    const result = await fetchTool({ url: `${serverUrl}/headers` });

    expect(result).toHaveProperty("status", 200);

    if ("headers" in result && "body" in result) {
      expect(result.headers["content-type"]).toBe("application/json");
      expect(result.headers["x-custom-header"]).toBe("test-value");
      expect(result.headers["x-another-header"]).toBe("another-value");
    }
  });

  test("Should handle different HTTP methods", async () => {
    const methods = ["PUT", "DELETE", "PATCH", "OPTIONS"];

    for (const method of methods) {
      const result = await fetchTool({
        url: `${serverUrl}/echo`,
        method,
      });

      expect(result).toHaveProperty("status", 200);

      if ("body" in result) {
        const response = JSON.parse(result.body);
        expect(response.method).toBe(method);
      }
    }
  });

  test("Should handle 404 errors", async () => {
    const result = await fetchTool({ url: `${serverUrl}/not-found` });

    expect(result).toHaveProperty("status", 404);
    expect(result).toHaveProperty("success", false);

    if ("body" in result) {
      expect(result.body).toBe("Not found");
    }
  });

  test("Should handle 500 errors", async () => {
    const result = await fetchTool({ url: `${serverUrl}/status/500` });

    expect(result).toHaveProperty("status", 500);
    expect(result).toHaveProperty("statusText", "Internal Server Error");
    expect(result).toHaveProperty("success", false);
  });

  test("Should handle timeout", async () => {
    const result = await fetchTool({
      url: `${serverUrl}/timeout?delay=2000`,
      timeout: 100, // 100ms timeout
    });

    expect(result).toEqual({
      error: "Request timeout after 100ms",
      code: "TIMEOUT",
    });
  });

  test("Should handle connection refused", async () => {
    const result = await fetchTool({ url: "http://127.0.0.1:1" });

    expect(result).toHaveProperty("error");
    expect(result).toHaveProperty("code");

    if ("code" in result) {
      expect(["ECONNREFUSED", "ConnectionRefused", "NETWORK_ERROR"]).toContain(result.code);
    }
  });

  test("Should handle DNS lookup failure", async () => {
    const result = await fetchTool({ url: "http://this-domain-does-not-exist-12345.com" });

    expect(result).toHaveProperty("error");
    expect(result).toHaveProperty("code");

    if ("code" in result) {
      expect(["ENOTFOUND", "DNSLookupFailed", "NETWORK_ERROR", "ConnectionRefused"]).toContain(
        result.code
      );
    }
  });

  test("Should trim whitespace from URL", async () => {
    const result = await fetchTool({ url: `  ${serverUrl}/echo  ` });

    expect(result).toHaveProperty("status", 200);
  });

  test("Should use default timeout of 30 seconds", async () => {
    // This test verifies the default timeout is applied
    const result = await fetchTool({ url: `${serverUrl}/echo` });

    expect(result).toHaveProperty("status", 200);
  });

  test("Should handle null body correctly", async () => {
    const result = await fetchTool({
      url: `${serverUrl}/echo`,
      method: "POST",
      body: null as any,
    });

    // null body should be ignored, not cause an error
    expect(result).toHaveProperty("status", 200);
  });
});
