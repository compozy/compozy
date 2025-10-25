<role>
You are a senior full-stack engineer. You will refactor the issue archiving system to change `archived` from a status enum value to a separate boolean field, covering both backend (Elysia + PostgreSQL + Drizzle) and frontend (React + TanStack Router + TanStack DB), following the project's established patterns and greenfield approach.
</role>

<dependent_tasks>
- Based on existing implementation: `packages/backend/src/db/schema/issues.ts` (schema), `packages/backend/src/modules/issues/*` (API), `packages/frontend/src/renderer/src/systems/issues/*` (frontend).
- Follows patterns from: TanStack Router for subroutes, TanStack DB collections for data fetching.
</dependent_tasks>

<context>
- Currently `archived` is one of 5 status values: `["planned", "in_progress", "blocked", "completed", "archived"]`
- Issues are archived by changing their status to "archived" via PATCH endpoint
- Frontend has `useArchiveIssue` hook that sets status to "archived"
- Archive button only appears for completed issues
- We need to change this to a cleaner architecture where `archived` is a separate boolean field
</context>

<scope>
Greenfield refactoring (no backwards compatibility needed):

**Backend Changes:**
- Add `archived` boolean column to `issues` table with default `false`
- Remove "archived" from `issue_status` enum (keep only 4 statuses)
- Update schema to include archived field and index
- Create new database migration
- Update repository to filter out archived issues by default in list queries
- Update API types and validation schemas
- Keep existing PATCH endpoint for status updates
- Add new PATCH endpoint for archive/unarchive operations

**Frontend Changes:**
- Add new `/archived` subroute using TanStack Router
- Create archived issues view (flat list, no status grouping)
- Add archive toggle button in top toolbar (same level as search/filter)
- Update types to include `archived: boolean` field
- Update issues collection to exclude archived by default
- Create separate archived collection for `/archived` route
- Update `useArchiveIssue` hook to set boolean instead of status
- Remove "archived" from status accordion sections
- Maintain restriction: only completed issues can be archived

**Database Migration:**
- Add `archived` boolean column (default false)
- Remove "archived" from enum (no data migration needed - greenfield)
- Add index on `archived` column for filtering performance
</scope>

<backend_requirements>

- Language/stack: TypeScript, Elysia 1+, Drizzle ORM, PostgreSQL, Zod.
- Schema changes (`packages/backend/src/db/schema/issues.ts`):
  - Remove "archived" from `issueStatuses` array (line 14-20)
  - Add `archived: boolean("archived").notNull().default(false)` to issues table
  - Add index: `index("idx_issues_archived").on(table.archived)`
  - Add composite index: `index("idx_issues_project_archived").on(table.projectId, table.archived)`
- Migration:
  - Create new migration file in `packages/backend/drizzle/`
  - Add `archived` boolean column with default false
  - Drop and recreate `issue_status` enum without "archived"
  - Update enum constraint on status column
- Repository changes (`packages/backend/src/modules/issues/repository.ts`):
  - Update `list()` to filter `WHERE archived = false` by default
  - Update `listForOrganization()` to filter `WHERE archived = false` by default
  - Add optional `includeArchived?: boolean` parameter to both methods
  - Update `mapIssue()` to include archived field
  - Add `archive(issueId: string)` method that sets `archived = true` (only if status = "completed")
  - Add `unarchive(issueId: string)` method that sets `archived = false`
- Model/Types changes (`packages/backend/src/modules/issues/model.ts`):
  - Remove "archived" from `issueStatuses` array
  - Add `archived: t.Boolean()` to `issueSchema`
  - Update `Issue` type to include `archived: boolean`
  - Keep `IssueStatus` type with only 4 values
- Route changes (`packages/backend/src/modules/issues/route.ts`):
  - Keep existing PATCH `/api/v1/projects/:projectId/issues/:issueId` for status updates
  - Add new PATCH `/api/v1/projects/:projectId/issues/:issueId/archive` endpoint
  - Add new PATCH `/api/v1/projects/:projectId/issues/:issueId/unarchive` endpoint
  - Archive endpoint validates: status must be "completed" (400 if not)
  - Add optional `?includeArchived=true` query param to list endpoints
- Validation:
  - Archive: `400` if issue not found or status !== "completed"
  - Unarchive: `400` if issue not found
  - `404` if issue doesn't exist
  - `500` generic errors
</backend_requirements>

<frontend_requirements>

- Stack: TypeScript, React 19, TanStack Router, TanStack DB, lucide-react.
- File naming: kebab-case for all files.

**Routing Changes:**
- Update existing route file: `packages/frontend/src/renderer/src/systems/issues/issues-route.tsx`
- Add nested routes in the same file (following project pattern):
  - `issuesRoute` - Parent route with layout (toolbar + `<Outlet />`)
  - `issuesIndexRoute` - Main list at `/issues`
  - `archivedIssuesRoute` - Archived list at `/issues/archived`
- Update `main.tsx` to register nested routes: `issuesRoute.addChildren([issuesIndexRoute, archivedIssuesRoute])`
- Archive button in toolbar uses `<Link to="/issues/archived">`
- Back button in archived view uses `<Link to="/issues">`

**Type Changes:**
- `packages/frontend/src/generated/api-schema.ts` is **auto-generated** from backend OpenAPI schema
- After backend changes, regenerate with: `cd packages/frontend && bun run generate:api-schema`
- This will automatically update:
  - `Issue` interface to include `archived: boolean`
  - `IssueStatus` union type to have only 4 values
  - `IssueListItem` to reflect new Issue shape

**Collection Changes:**
- `packages/frontend/src/renderer/src/systems/issues/db/issues-collection.ts`:
  - Update `createIssuesCollection()` to fetch only non-archived issues
  - Remove "archived" from status filter options (line 10)
- Create new collection file: `packages/frontend/src/renderer/src/systems/issues/db/archived-issues-collection.ts`
  - Similar to issues-collection but fetches only archived issues
  - No status filtering needed (flat list)
  - Uses endpoint with `?includeArchived=true` or separate archived endpoint

**Hook Changes:**
- `packages/frontend/src/renderer/src/systems/issues/hooks/use-archive-issue.ts`:
  - Change implementation from `collection.update(issueId, draft => { draft.issue.status = "archived" })`
  - To: Call new archive API endpoint directly (not via collection.update)
  - Return mutation with loading/error states
  - After success, invalidate issues query
- Create new hook: `use-archived-issues.ts` for archived view
  - Similar pattern to `use-issues.ts`
  - Uses `createArchivedIssuesCollection()`
- Create new hook: `use-unarchive-issue.ts`
  - Calls unarchive endpoint
  - Invalidates archived issues query after success

**Component Changes:**
- `packages/frontend/src/renderer/src/systems/issues/components/issue-list.tsx`:
  - Remove "archived" from `statusVariants` object (line 48-61)
  - Remove "archived" from `IssueListSkeleton` statuses array (line 141)
  - Keep archive button in ItemActions (already conditionally rendered for completed)
  - Update `useIssuesByStatus` to handle only 4 statuses
- Create new file: `packages/frontend/src/renderer/src/systems/issues/issues-layout.tsx`
  - Contains shared toolbar (search, filter, archive toggle button)
  - Renders `<Outlet />` for nested routes
  - Archive button uses `<Link to="/issues/archived">`
- Create new file: `packages/frontend/src/renderer/src/systems/issues/archived-issues.tsx`
  - Main component for archived view
  - Uses `useArchivedIssues()` hook
  - Renders `<ArchivedIssueList>` component
  - Back button: `<Link to="/issues">`
- Create new component: `packages/frontend/src/renderer/src/systems/issues/components/archived-issue-list.tsx`
  - Flat list of archived issues (no accordion, no status grouping)
  - Simple `<Item>` components in `<ItemGroup>`
  - Show issue title, original status badge, unarchive button
  - Include search functionality
  - No project filter (show all archived issues from all projects)

**Route Structure (all in `issues-route.tsx`):**
- `issuesRoute` - Parent layout with toolbar + `<Outlet />`
- `issuesIndexRoute` - Renders `<Issues>` component at `/issues`
- `archivedIssuesRoute` - Renders `<ArchivedIssues>` component at `/issues/archived`
</frontend_requirements>

<database_migration>

**Note:** Drizzle ORM automatically generates migrations based on schema changes.

After updating the schema in `packages/backend/src/db/schema/issues.ts`, run:

```bash
cd packages/backend
bun run db:generate  # Generates migration files automatically
bun run db:migrate   # Applies migrations to database
```

Drizzle will generate a migration that:
- Adds `archived` boolean column with default `false`
- Adds indexes on `archived` column
- Drops and recreates `issue_status` enum without "archived" value
- Updates all type constraints

**Manual verification recommended:**
- Review generated migration in `packages/backend/drizzle/` before applying
- Test migration on development database first
- Verify enum changes don't cause issues with existing queries

</database_migration>

<api_endpoints>

**Existing (Updated):**
```
GET /api/v1/projects/:projectId/issues
  - Add optional query param: ?includeArchived=true
  - Default behavior: returns only non-archived issues
  - Response includes archived: boolean field

GET /api/v1/organizations/:orgId/issues
  - Add optional query param: ?includeArchived=true
  - Default behavior: returns only non-archived issues

PATCH /api/v1/projects/:projectId/issues/:issueId
  - Keep existing (updates status, title, summary, etc.)
  - Does NOT update archived field
```

**New:**
```
PATCH /api/v1/projects/:projectId/issues/:issueId/archive
  - Body: none (or empty {})
  - Validates: issue.status === "completed"
  - Sets: archived = true
  - Returns: 200 with updated issue
  - Errors: 400 if not completed, 404 if not found

PATCH /api/v1/projects/:projectId/issues/:issueId/unarchive
  - Body: none (or empty {})
  - Sets: archived = false
  - Returns: 200 with updated issue
  - Errors: 404 if not found
```

</api_endpoints>

<request_examples>

**Archive a completed issue:**
```bash
curl -X PATCH http://localhost:3005/api/v1/projects/<PROJECT_ID>/issues/<ISSUE_ID>/archive
```

**Unarchive an issue:**
```bash
curl -X PATCH http://localhost:3005/api/v1/projects/<PROJECT_ID>/issues/<ISSUE_ID>/unarchive
```

**List issues (excludes archived by default):**
```bash
curl http://localhost:3005/api/v1/projects/<PROJECT_ID>/issues
```

**List ALL issues including archived:**
```bash
curl http://localhost:3005/api/v1/projects/<PROJECT_ID>/issues?includeArchived=true
```

</request_examples>

<acceptance_criteria>

**Backend:**
- Schema has `archived` boolean column with proper indexes
- `issue_status` enum has only 4 values (no "archived")
- Repository filters archived issues by default
- Archive endpoint validates status="completed" before archiving
- Unarchive endpoint works without status restrictions
- All existing tests pass
- Migration runs successfully

**Frontend:**
- `/issues` route shows only non-archived issues
- `/issues/archived` route shows flat list of archived issues
- Archive button appears in top toolbar and navigates to archived route
- Archive functionality works with optimistic updates
- Unarchive button appears in archived list
- Status accordion has only 4 sections (no "archived")
- Types correctly reflect new schema
- No "key not found" errors in collections
- Proper loading/error states

**Database:**
- Migration adds archived column
- Enum recreated without "archived"
- Indexes created for performance
- No data loss (greenfield - no existing archived issues)

</acceptance_criteria>

<suggested_steps>

1. **Backend Schema & Migration**
   - Update `packages/backend/src/db/schema/issues.ts`:
     - Remove "archived" from `issueStatuses` array
     - Add `archived` boolean column
     - Add indexes for archived column
   - Generate migration: `cd packages/backend && bun run db:generate`
   - Review generated migration in `packages/backend/drizzle/`
   - Apply migration: `bun run db:migrate`
   - Verify schema changes: `bun run db:studio`

2. **Backend Repository**
   - Update `packages/backend/src/modules/issues/repository.ts`:
     - Add archived field to `mapIssue()` function
     - Update `list()` to add `eq(issues.archived, false)` to where clause
     - Update `listForOrganization()` similarly
     - Add optional `includeArchived` parameter
     - Add `archive()` method with validation
     - Add `unarchive()` method
   - Update `packages/backend/src/modules/issues/model.ts`:
     - Remove "archived" from `issueStatuses`
     - Add `archived: t.Boolean()` to schema
     - Update types

3. **Backend API Routes**
   - Update `packages/backend/src/modules/issues/route.ts`:
     - Add `/archive` endpoint (PATCH)
     - Add `/unarchive` endpoint (PATCH)
     - Add `includeArchived` query param to list endpoints
     - Add validation for archive (must be completed)
   - Update usecases if needed

4. **Backend Testing**
   - Run `bun typecheck` to verify types
   - Run `bun test` to ensure existing tests pass
   - Manually test archive/unarchive endpoints
   - Verify archived issues filtered by default

5. **Frontend Types (Auto-Generated)**
   - Regenerate API schema from backend OpenAPI spec:
     ```bash
     cd packages/frontend
     bun run generate:api-schema
     ```
   - This automatically updates `packages/frontend/src/generated/api-schema.ts`
   - Verify generated types:
     - `archived: boolean` in Issue type
     - `IssueStatus` has only 4 values
   - No manual type changes needed

6. **Frontend Collections**
   - Update `packages/frontend/src/renderer/src/systems/issues/db/issues-collection.ts`:
     - Remove "archived" from status filter type
   - Create `packages/frontend/src/renderer/src/systems/issues/db/archived-issues-collection.ts`:
     - Fetch issues with `includeArchived=true` and filter `archived === true`
     - Or create separate endpoint for archived issues

7. **Frontend Hooks**
   - Update `packages/frontend/src/renderer/src/systems/issues/hooks/use-archive-issue.ts`:
     - Change from collection.update to direct API call
     - Call `/archive` endpoint
     - Return mutation with proper states
   - Create `use-archived-issues.ts` hook
   - Create `use-unarchive-issue.ts` hook

8. **Frontend Routing**
   - Update `packages/frontend/src/renderer/src/systems/issues/issues-route.tsx`:
     - Export `issuesRoute` (parent with layout)
     - Export `issuesIndexRoute` (main list at `/issues`)
     - Export `archivedIssuesRoute` (archived at `/issues/archived`)
   - Create `issues-layout.tsx` component (toolbar + `<Outlet />`)
   - Create `archived-issues.tsx` component
   - Update `main.tsx` to register nested routes

9. **Frontend Components**
   - Update `issue-list.tsx`:
     - Remove "archived" from statusVariants
     - Remove from skeleton
   - Create `archived-issue-list.tsx`:
     - Flat list component
     - Show title, status badge, unarchive button
     - Include search
   - Update `use-issues-by-status.ts`:
     - Remove "archived" from Record type

10. **Frontend Testing**
    - Run `bun lint` to check code quality
    - Run `bun typecheck` to verify types
    - Run `bun test` for unit tests
    - Manual testing:
      - Create completed issue
      - Archive it (verify navigation to /archived)
      - Check /issues route (should not show archived)
      - Check /archived route (should show it)
      - Unarchive it
      - Verify it reappears in main list
      - Test archive button only on completed issues

11. **Integration Testing**
    - Test full flow: create → complete → archive → unarchive
    - Verify filters work correctly
    - Check collection updates properly
    - Verify no console errors
    - Test with multiple projects
    - Verify archived issues don't appear in status filters

12. **Quality Checks**
    - Run `bun lint` - must pass
    - Run `bun typecheck` - must pass
    - Run `bun test` - must pass
    - Manual smoke testing of all functionality
    - Verify no breaking changes to existing features

</suggested_steps>

<best_practices>

- **Greenfield Approach**: Don't worry about backwards compatibility - make clean architectural decisions
- **Separation of Concerns**: Archived is state management, not status - boolean is cleaner
- **Type Safety**: Leverage TypeScript to catch issues early
- **Database Indexes**: Add indexes on filtered columns (archived) for performance
- **API Design**: Separate endpoints for archive operations vs status updates (clear intent)
- **Default Behavior**: Filter archived by default (explicit opt-in to see them)
- **Validation**: Only completed issues can be archived (business rule enforcement)
- **Optimistic Updates**: Provide instant UI feedback where possible
- **Route-Based Views**: Use TanStack Router for clean separation of main vs archived views
- **Flat Architecture**: Archived view doesn't need status grouping (simpler UI)
- **Error Handling**: Proper error states and rollback mechanisms
- **Testing**: Comprehensive testing at each layer (DB, API, hooks, components)

</best_practices>

<should_not>

- Don't maintain backwards compatibility (greenfield project)
- Don't allow archiving non-completed issues (enforce business rule)
- Don't include archived issues in main list by default (requires explicit opt-in)
- Don't use status for archiving (that's what caused the problem)
- Don't forget to add database indexes (performance)
- Don't skip migration (schema must match code)
- Don't mix archive and status update logic in same endpoint
- Don't forget to update all types (backend and frontend)
- Don't create new collection instances in mutation hooks (share collections)
- Don't forget to invalidate queries after mutations
- Don't add "include archived" checkbox (clean UI - separate route instead)
- Don't group archived issues by status (flat list is simpler)
- Don't forget aria-labels for accessibility

</should_not>

<references>
- Drizzle ORM Migrations: https://orm.drizzle.team/docs/migrations
- PostgreSQL Enums: https://www.postgresql.org/docs/current/datatype-enum.html
- Elysia Route Patterns: @.cursor/rules/elysia.mdc
- TanStack Router Nested Routes: @.cursor/rules/tanstack-router.mdc
- TanStack DB Collections: @.cursor/rules/data-fetch.mdc
- React Component Patterns: @.cursor/rules/react.mdc
- Existing Issue System: packages/backend/src/modules/issues/* and packages/frontend/src/renderer/src/systems/issues/*
</references>

<relevant_files>
**Backend:**
- packages/backend/src/db/schema/issues.ts (schema definition)
- packages/backend/drizzle/* (migrations)
- packages/backend/src/modules/issues/model.ts (types)
- packages/backend/src/modules/issues/repository.ts (data access)
- packages/backend/src/modules/issues/route.ts (API endpoints)
- packages/backend/src/modules/issues/usecases.ts (business logic)

**Frontend:**
- packages/frontend/src/generated/api-schema.ts (generated types)
- packages/frontend/src/renderer/src/systems/issues/issues-route.tsx (nested routes)
- packages/frontend/src/renderer/src/systems/issues/issues-layout.tsx (layout with outlet)
- packages/frontend/src/renderer/src/systems/issues/archived-issues.tsx (archived view)
- packages/frontend/src/renderer/src/systems/issues/db/issues-collection.ts
- packages/frontend/src/renderer/src/systems/issues/hooks/use-archive-issue.ts
- packages/frontend/src/renderer/src/systems/issues/hooks/use-issues-by-status.ts
- packages/frontend/src/renderer/src/systems/issues/components/issue-list.tsx
- packages/frontend/src/renderer/src/systems/issues/components/archived-issue-list.tsx
- packages/frontend/src/renderer/src/main.tsx (route tree registration)
</relevant_files>

<dependent_files>
- packages/backend/src/db/index.ts (database client)
- packages/frontend/src/lib/api/* (API client)
- packages/frontend/src/lib/query-client.ts (TanStack Query config)
</dependent_files>

<output>
This refactoring creates a cleaner separation between issue workflow status and archival state. By making `archived` a boolean field instead of a status value, we:

1. **Improve Semantics**: Status represents workflow state; archived represents visibility state
2. **Simplify Logic**: Clear business rule (only completed issues can be archived)
3. **Better UX**: Separate route for archived issues with simplified flat view
4. **Type Safety**: Smaller status enum (4 values) reduces complexity
5. **Performance**: Proper indexes on archived column for efficient filtering
6. **Maintainability**: Clearer intent with dedicated archive/unarchive endpoints

**Key Architectural Decisions:**
- Separate API endpoints for archive operations (clear intent)
- Default filtering of archived issues (explicit opt-in)
- Route-based navigation for archived view (clean separation)
- Flat list for archived issues (no status grouping needed)
- Validation at API layer (only completed → archived)

**Testing Focus:**
- Database migration success
- Repository filtering works correctly
- Archive validation prevents non-completed archiving
- Frontend routing and navigation
- Collection invalidation after mutations
- No regression in existing functionality
</output>

<perplexity>
- Use perplexity and context7 to research:
  - Drizzle ORM ALTER TYPE/ENUM patterns
  - PostgreSQL enum migration best practices
  - TanStack Router nested route patterns
  - TanStack DB collection invalidation strategies
</perplexity>

<greenfield>
**YOU SHOULD ALWAYS** have in mind that this should be done in a greenfield approach. We don't need to care about backwards compatibility since the project is in alpha, and supporting old and new stuff just introduces more complexity in the project; never sacrifice quality because of backwards compatibility.
</greenfield>

<rules>
**CRITICAL: Read these rule files before starting implementation:**

Before implementing ANY part of this refactoring, you MUST read and follow these rules:

**Backend Rules:**
1. `.cursor/rules/no-linebreaks.mdc` - MOST IMPORTANT: No linebreaks in functions
2. `.cursor/rules/elysia.mdc` - Elysia API patterns and best practices

**Frontend Rules:**
1. `.cursor/rules/no-linebreaks.mdc` - MOST IMPORTANT: No linebreaks in functions
2. `.cursor/rules/react.mdc` - React component patterns and hooks
3. `.cursor/rules/tanstack-router.mdc` - TanStack Router nested routing patterns
4. `.cursor/rules/data-fetch.mdc` - TanStack DB collections and query patterns (CRITICAL for collection sharing)
5. `.cursor/rules/shadcn.mdc` - ShadCN component usage
6. `.cursor/rules/tailwindcss.mdc` - Tailwind CSS design tokens (use bg-background, NOT bg-white)
7. `.cursor/rules/zustand.mdc` - State management (if needed)
8. `.cursor/rules/tanstack-forms.mdc` - Form handling (if needed)

**Most Critical Rules:**
- ⚠️ **Collection Sharing** (data-fetch.mdc): ALWAYS share the same collection instance between hooks - creating new instances causes "key not found" errors
- ⚠️ **No Linebreaks** (no-linebreaks.mdc): Functions must have NO extra linebreaks between statements
- ⚠️ **Design Tokens** (tailwindcss.mdc): NEVER use explicit colors (bg-white, text-black), ALWAYS use tokens (bg-background, text-foreground)
- ⚠️ **Nested Routes** (tanstack-router.mdc): Use `<Outlet />` pattern for nested routes as shown in the plan

**Enforcement:**
Violating these standards results in immediate task rejection. Code that doesn't follow these rules will NOT be accepted.
</rules>
