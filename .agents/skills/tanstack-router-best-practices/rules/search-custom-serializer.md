# search-custom-serializer: Configure Custom Search Param Serializers

## Priority: LOW

## Explanation

By default, TanStack Router serializes search params as JSON. For cleaner URLs or compatibility with external systems, you can provide custom serializers using libraries like `qs`, `query-string`, or your own implementation.

## Bad Example

```tsx
// Default JSON serialization creates ugly URLs
// URL: /products?filters=%7B%22category%22%3A%22electronics%22%2C%22inStock%22%3Atrue%7D

// Or manually parsing/serializing inconsistently
function ProductList() {
  const searchParams = new URLSearchParams(window.location.search);
  const filters = JSON.parse(searchParams.get("filters") || "{}");
  // Inconsistent with router's handling
}
```

## Good Example: Using JSURL for Compact URLs

```tsx
import { createRouter } from "@tanstack/react-router";
import JSURL from "jsurl2";

const router = createRouter({
  routeTree,
  search: {
    // Custom serializer for compact, URL-safe encoding
    serialize: search => JSURL.stringify(search),
    parse: searchString => JSURL.parse(searchString) || {},
  },
});

// URL: /products?~(category~'electronics~inStock~true)
// Much shorter than JSON!
```

## Good Example: Using query-string for Flat Params

```tsx
import { createRouter } from "@tanstack/react-router";
import queryString from "query-string";

const router = createRouter({
  routeTree,
  search: {
    serialize: search =>
      queryString.stringify(search, {
        arrayFormat: "bracket",
        skipNull: true,
      }),
    parse: searchString =>
      queryString.parse(searchString, {
        arrayFormat: "bracket",
        parseBooleans: true,
        parseNumbers: true,
      }),
  },
});

// URL: /products?category=electronics&inStock=true&tags[]=sale&tags[]=new
// Traditional query string format
```

## Good Example: Using qs for Nested Objects

```tsx
import { createRouter } from "@tanstack/react-router";
import qs from "qs";

const router = createRouter({
  routeTree,
  search: {
    serialize: search =>
      qs.stringify(search, {
        encodeValuesOnly: true,
        arrayFormat: "brackets",
      }),
    parse: searchString =>
      qs.parse(searchString, {
        ignoreQueryPrefix: true,
        decoder(value) {
          // Parse booleans and numbers
          if (value === "true") return true;
          if (value === "false") return false;
          if (/^-?\d+$/.test(value)) return parseInt(value, 10);
          return value;
        },
      }),
  },
});

// URL: /products?filters[category]=electronics&filters[price][min]=100&filters[price][max]=500
```

## Good Example: Base64 for Complex State

```tsx
import { createRouter } from "@tanstack/react-router";

const router = createRouter({
  routeTree,
  search: {
    serialize: search => {
      if (Object.keys(search).length === 0) return "";
      const json = JSON.stringify(search);
      return btoa(json); // Base64 encode
    },
    parse: searchString => {
      if (!searchString) return {};
      try {
        return JSON.parse(atob(searchString)); // Base64 decode
      } catch {
        return {};
      }
    },
  },
});

// URL: /products?eyJjYXRlZ29yeSI6ImVsZWN0cm9uaWNzIn0
// Opaque but compact
```

## Good Example: Hybrid Approach

```tsx
// Some params as regular query, complex ones as JSON
import { createRouter } from "@tanstack/react-router";

const router = createRouter({
  routeTree,
  search: {
    serialize: search => {
      const { filters, ...simple } = search;
      const params = new URLSearchParams();

      // Simple values as regular params
      Object.entries(simple).forEach(([key, value]) => {
        if (value !== undefined) {
          params.set(key, String(value));
        }
      });

      // Complex filters as JSON
      if (filters && Object.keys(filters).length > 0) {
        params.set("filters", JSON.stringify(filters));
      }

      return params.toString();
    },
    parse: searchString => {
      const params = new URLSearchParams(searchString);
      const result: Record<string, unknown> = {};

      params.forEach((value, key) => {
        if (key === "filters") {
          result.filters = JSON.parse(value);
        } else if (value === "true") {
          result[key] = true;
        } else if (value === "false") {
          result[key] = false;
        } else if (/^-?\d+$/.test(value)) {
          result[key] = parseInt(value, 10);
        } else {
          result[key] = value;
        }
      });

      return result;
    },
  },
});

// URL: /products?page=1&sort=price&filters={"category":"electronics","inStock":true}
```

## Serializer Comparison

| Library        | URL Style               | Best For          |
| -------------- | ----------------------- | ----------------- |
| Default (JSON) | `?data=%7B...%7D`       | TypeScript safety |
| jsurl2         | `?~(key~'value)`        | Compact, readable |
| query-string   | `?key=value&arr[]=1`    | Traditional APIs  |
| qs             | `?obj[nested]=value`    | Deep nesting      |
| Base64         | `?eyJrZXkiOiJ2YWx1ZSJ9` | Opaque, compact   |

## Context

- Custom serializers apply globally to all routes
- Route-level `validateSearch` still works after parsing
- Consider URL length limits (~2000 chars for safe cross-browser)
- SEO: Search engines may not understand custom formats
- Bookmarkability: Users can't easily modify opaque URLs
- Debugging: JSON is easier to read in browser devtools
