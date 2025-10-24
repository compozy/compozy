<role>
You are a senior software engineer. You will implement an MVP for uploading **product** images in the existing monorepo, covering backend (Hono + SQLite + Bun) and frontend (React/Vite), following the simple pattern from previous tasks and WITHOUT changing the `Product` schema.
</role>

<dependent_tasks>
- Base on previous tasks: `@tasks/task_1.md` (backend), `@tasks/task_2.md` (frontend) and `@tasks/task_3.md` (edit/remove).
</dependent_tasks>

<context>
- The `product_images` table already exists in the backend and seed scripts that copy images to `backend/uploads/products`.
- We need to expose `/uploads/...` statically and create image endpoints per product.
- In the frontend, show only the "cover" (first image by `position`) in the `ProductCard`. Simple upload, multiple files, in create/edit dialogs.
</context>

<scope>
MVP, keeping the project minimalist:

- Local storage in `backend/uploads/products` and serve statically via `/uploads/...`.
- `POST /api/products/:id/images` accepts only `multipart/form-data` with `images` field (can send multiple files). Don't support "add by URL" in this MVP.
- When deleting image/product, also remove the local file from disk (when the URL is local).
- Frontend: display only the cover (first image) in the card; simple multiple upload in create/edit dialogs. No gallery, no reordering.
- In dev, also proxy `'/uploads'` in `@frontend/vite.config.ts`.
- Default limits: up to 5 files per request; up to 5MB per file; validate mimetype (`image/jpeg`, `image/png`, `image/webp`).
</scope>

<backend_requirements>
- Language/stack: TypeScript, Hono 4+, Bun, Zod (same project pattern).
- Serve static files:
  - Expose `backend/uploads` via `/uploads` (e.g.: `GET /uploads/products/abc.jpg`).
  - Ensure directory creation with `fs.mkdirSync(uploadsDir, { recursive: true })`.
- Image endpoints (per product):
  - `GET /api/products/:id/images` → list product images (sort by `position ASC, createdAt ASC`).
  - `POST /api/products/:id/images` → upload multiple files in the `images` field (multipart). For each valid file:
    - Validate size and mimetype.
    - Generate unique name (UUID + normalized extension) and save in `backend/uploads/products`.
    - Persist in `product_images` with relative `url` starting with `/uploads/products/...` and incremental `position` (append to end).
    - Return 201 with simple payload of created images.
  - `DELETE /api/products/:id/images/:imageId` → delete row and, if the URL is local (`/uploads/products/...`), remove file from disk.
- Errors and validation (follow project pattern):
  - `400` validation (invalid ID, no files, empty file, etc.).
  - `404` product/image not found.
  - `413` file too large (> 5MB).
  - `415` unsupported mimetype.
  - `500` generic.
- Notes:
  - DO NOT change the base `Product` schema nor the `/api/products` response.
  - Use relative URLs (starting with `/uploads/...`).
  - Sanitize extension from detected mimetype; don't trust original name.
</backend_requirements>

<frontend_requirements>
- Stack: TypeScript, Vite, React Query, react-hook-form, Zod and `@/components/ui/*` components.
- Vite Proxy: in addition to `'/api'`, add proxy for `'/uploads'` pointing to backend.
- Types:
  - Keep `Product` unchanged.
  - Create local schema/typing for image (e.g.: `{ id: string; url: string; position: number; createdAt: string }`).
- Hooks:
  - `useUploadProductImages(productId)` with `useMutation` for `POST /api/products/:id/images` using `FormData` with multiple files.
  - (Optional and simple) `useProductImages(productId)` with `useQuery` for `GET /api/products/:id/images`, if needed to fetch cover later.
- UI:
  - `ProductCard`: render only the cover (first image). If none exists, show placeholder.
  - `AddProductDialog`/`EditProductDialog`: multiple upload field (can use `@/components/ui/kibo-ui/dropzone` or `<input type="file" multiple>`). After create/edit, if there are selected files, call `useUploadProductImages` and invalidate `['products']` (and/or images cache) at the end.
  - No gallery and no reordering in this MVP.
- Requests always with relative paths (`/api/...` and `/uploads/...`) via proxy.
</frontend_requirements>

<request_examples>
- Multipart upload (multiple files):

```bash
curl -X POST http://localhost:3005/api/products/ \
  -F "images=@/path/image1.jpg" \
  -F "images=@/path/image2.png" < PRODUCT_ID > /images
```

- List product images:

```bash
curl http://localhost:3005/api/products/ < PRODUCT_ID > /images
```

- Delete specific image:

```bash
curl -X DELETE http://localhost:3005/api/products/<PRODUCT_ID>/images/<IMAGE_ID>
```
</request_examples>

<acceptance_criteria>
- Backend:
  - Serve `/uploads/...` working locally.
  - `POST /api/products/:id/images` saves valid files to disk, creates rows and returns 201.
  - `GET /api/products/:id/images` returns ordered list.
  - `DELETE /api/products/:id/images/:imageId` removes row and local file.
  - Limits and validations active (size, mimetype, quantity).
- Frontend:
  - `ProductCard` displays cover when it exists; placeholder otherwise.
  - Create/edit dialogs support selecting multiple images and uploading after saving the product.
  - `'/uploads'` proxy configured; `<img src="/uploads/...">` works in dev.
  - Simple loading/error states in mutations.
</acceptance_criteria>

<suggested_steps>
1. Backend
   - Add static middleware for `/uploads` and ensure existence of `backend/uploads/products`.
   - Implement image endpoints in products router (`GET/POST/DELETE`).
   - Validate limits (up to 5 files, 5MB each, allowed mimetypes). Generate names with UUID and write to disk.
   - When deleting image/product, remove local file when applicable.

2. Frontend
   - Add proxy for `'/uploads'` in `vite.config.ts`.
   - Adjust `ProductCard` to fetch/display simple cover (first image) — or derive from response if already in cache.
   - Update dialogs to allow file selection; after save/edit, send via `useUploadProductImages`.
   - Invalidate `['products']` (and/or images cache) after success.

3. Manual smoke test
   - Create product → upload 1–2 images → see cover in grid → delete one image → confirm visual removal and on disk.

4. (Optional) Basic integration tests
   - `POST` with valid and invalid file (mimetype/size); `GET` list; `DELETE` removes row and file.
</suggested_steps>

<best_practices>
- **Relative URLs** for files (`/uploads/...`), never absolute filesystem paths.
- **Validate real mimetype** (don't trust extension only). Reject unsupported types.
- **Generate unique name** with UUID and maintain extension coherent with mimetype.
- **Sanitize** names/paths and block path traversal.
- **Consistent JSON responses** and correct HTTP codes.
- **Avoid new dependencies** in this MVP (no thumbnails/transformations).
- **Invalidate caches** from React Query after mutations.
</best_practices>

<should_not>
- Don't change the `Product` schema nor change the `/api/products` contract.
- Don't accept "add by URL" in this MVP.
- Don't hardcode host/port in frontend; use relative paths and Vite proxy.
- Don't trust only the file extension; validate mimetype.
- Don't save absolute disk paths in database; save relative URLs.
- Don't allow unlimited uploads (apply size and quantity limits).
- Don't embed images in Base64 in API JSON.
</should_not>

<references>
- Hono – Uploads/multipart and FormData: [Hono Documentation](https://hono.dev/)
- Hono – Serve static (`serveStatic` – Bun): [Hono Bun Examples](https://hono.dev/getting-started/bun)
- Bun FS (files): [Bun.serve and FS](https://bun.sh/docs/api/fs)
- Vite Proxy: [Vite Proxy Configuration](https://vitejs.dev/config/server-options.html#server-proxy)
- MDN – `multipart/form-data`: [FormData MDN](https://developer.mozilla.org/docs/Web/API/FormData)
</references>

<relevant_files>
- `path/to/file.go`
</relevant_files>

<dependent_files>
- `path/to/dependency.go`
</dependent_files>

<output>
In addition to the code, provide a brief summary of what was implemented, upload request examples and note decisions/limits applied (quantity, size, mimetypes) in the backend README.
</output>

<perplexity>
- use perplexity and context7 to find out how to properly fix/resolve this
- when using perplexity mcp, you can pass a prompt to the query param with more description about what you want to know, you don't need to pass a query-style search phrase
- for context7 to use the mcp is two steps, one you will find out the library id and them you will check what you want
</perplexity>

<greenfield>
**YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach, we don't need to care about backwards compatibility since the project is in alpha, and support old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility
</greenfield>

<output>
In addition to the code, provide a brief summary of what was implemented, upload request examples and note decisions/limits applied (quantity, size, mimetypes) in the backend README.
</output>
