# search-validation: Always Validate Search Params

## Priority: HIGH

## Explanation

Search params come from the URL - user-controlled input that must be validated. Use `validateSearch` to parse, validate, and provide defaults. This ensures type safety and prevents runtime errors from malformed URLs.

## Bad Example

```tsx
// No validation - trusting URL input directly
export const Route = createFileRoute("/products")({
  component: ProductsPage,
});

function ProductsPage() {
  // Accessing raw search params without validation
  const searchParams = new URLSearchParams(window.location.search);
  const page = parseInt(searchParams.get("page") || "1"); // Could be NaN
  const sort = searchParams.get("sort") as "asc" | "desc"; // Could be anything

  // Runtime errors possible if URL is malformed
  return <ProductList page={page} sort={sort} />;
}
```

## Good Example: Manual Validation

```tsx
export const Route = createFileRoute("/products")({
  validateSearch: (search: Record<string, unknown>) => {
    return {
      page: Number(search.page) || 1,
      sort: search.sort === "desc" ? "desc" : "asc",
      category: typeof search.category === "string" ? search.category : undefined,
      minPrice: Number(search.minPrice) || undefined,
      maxPrice: Number(search.maxPrice) || undefined,
    };
  },
  component: ProductsPage,
});

function ProductsPage() {
  // Fully typed, validated search params
  const { page, sort, category, minPrice, maxPrice } = Route.useSearch();
  // page: number (default 1)
  // sort: 'asc' | 'desc' (default 'asc')
  // category: string | undefined
}
```

## Good Example: With Zod

```tsx
import { z } from "zod";

const productSearchSchema = z.object({
  page: z.number().min(1).catch(1),
  limit: z.number().min(1).max(100).catch(20),
  sort: z.enum(["name", "price", "date"]).catch("name"),
  order: z.enum(["asc", "desc"]).catch("asc"),
  category: z.string().optional(),
  search: z.string().optional(),
  minPrice: z.number().min(0).optional(),
  maxPrice: z.number().min(0).optional(),
});

type ProductSearch = z.infer<typeof productSearchSchema>;

export const Route = createFileRoute("/products")({
  validateSearch: search => productSearchSchema.parse(search),
  component: ProductsPage,
});

function ProductsPage() {
  const search = Route.useSearch();
  // search: ProductSearch - fully typed with defaults

  return (
    <ProductList
      page={search.page}
      limit={search.limit}
      sort={search.sort}
      order={search.order}
      filters={{
        category: search.category,
        search: search.search,
        priceRange:
          search.minPrice && search.maxPrice ? [search.minPrice, search.maxPrice] : undefined,
      }}
    />
  );
}
```

## Good Example: With Valibot

```tsx
import * as v from "valibot";
import { valibotSearchValidator } from "@tanstack/router-valibot-adapter";

const searchSchema = v.object({
  page: v.fallback(v.number(), 1),
  query: v.fallback(v.string(), ""),
  filters: v.fallback(v.array(v.string()), []),
});

export const Route = createFileRoute("/search")({
  validateSearch: valibotSearchValidator(searchSchema),
  component: SearchPage,
});
```

## Updating Search Params

```tsx
function ProductFilters() {
  const navigate = useNavigate();
  const search = Route.useSearch();

  const updateFilters = (newFilters: Partial<ProductSearch>) => {
    navigate({
      to: ".", // Current route
      search: prev => ({
        ...prev,
        ...newFilters,
        page: 1, // Reset to page 1 when filters change
      }),
    });
  };

  return (
    <div>
      <select
        value={search.sort}
        onChange={e => updateFilters({ sort: e.target.value as ProductSearch["sort"] })}
      >
        <option value="name">Name</option>
        <option value="price">Price</option>
        <option value="date">Date</option>
      </select>
    </div>
  );
}
```

## Context

- Search params are user input - never trust them unvalidated
- Use `.catch()` in Zod or `fallback()` in Valibot for graceful defaults
- Validation runs on every navigation - keep it fast
- Search params are inherited by child routes
- Use `search` updater function to preserve other params
