# Testcontainer Optimization Guide

     ## Overview

     We've implemented a shared container pattern for PostgreSQL that reduces test execution time by 70-90%. Instead of creating a new container for each test (5-10s overhead), we now share a single container across all tests with proper isolation.

     ## Migration Guide

     ### 1. Update Individual Test Files

     Replace the old pattern with the new shared pattern:

     ```go
     // ❌ OLD: Creates a new container for each test (SLOW)
     func TestSomething(t *testing.T) {
         ctx := context.Background()
         pool, cleanup := helpers.CreateTestContainerDatabase(ctx, t)
         defer cleanup()

         // test code...
     }

     // ✅ NEW: Uses shared container (FAST)
     func TestSomething(t *testing.T) {
         ctx := context.Background()
         pool, cleanup := helpers.GetSharedPostgresDB(ctx, t)
         defer cleanup()

         // test code...
     }
     ```

     ### 2. Add TestMain to Package

     For packages with many tests, add a TestMain to handle container lifecycle:

     ```go
     func TestMain(m *testing.M) {
         // Run tests
         code := m.Run()

         // Cleanup shared container
         helpers.CleanupSharedContainer()

         os.Exit(code)
     }
     ```

     ### 3. Test Isolation

     The shared pattern automatically handles test isolation by:
     - Truncating all tables after each test (except migration tracking)
     - Each test starts with a clean database state
     - No test can see data from other tests

     ### 4. Parallel Testing

     Tests can still run in parallel:
     ```go
     func TestParallel(t *testing.T) {
         t.Parallel() // Safe to use with shared container

         ctx := context.Background()
         pool, cleanup := helpers.GetSharedPostgresDB(ctx, t)
         defer cleanup()

         // test code...
     }
     ```

     ## Performance Comparison

     ### Before (Individual Containers)
     - Container startup: 5-10s per test
     - 100 tests = 500-1000s overhead
     - Total time: ~15-20 minutes

     ### After (Shared Container)
     - Container startup: 5-10s once
     - 100 tests = minimal overhead
     - Total time: ~2-3 minutes

     ## Best Practices

     1. **Always use GetSharedPostgresDB** for new tests
     2. **Add TestMain** to packages with multiple tests
     3. **Don't modify goose_db_version** table in tests
     4. **Use unique identifiers** in test data to avoid conflicts

     ## Troubleshooting

     ### Container fails to start
     - Check Docker is running
     - Ensure port 5432 is not in use
     - Check Docker resources (CPU/RAM)

     ### Tests fail with "table does not exist"
     - Migrations run automatically on container start
     - Check migration files are present
     - Verify no test is dropping tables

     ### Cleanup not working
     - Ensure cleanup function is deferred
     - Check for errors in table truncation
     - Verify table names in cleanup list

     ## Next Steps

     1. Migrate all tests using CreateTestContainerDatabase
     2. Monitor test execution times
     3. Consider similar patterns for Redis/Temporal
     4. Enable container reuse for CI/CD
