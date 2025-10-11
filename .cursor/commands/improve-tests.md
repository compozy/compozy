# Compozy Test Improvement Prompt Template

<directory>$ARGUMENTS</directory>

<critical>
- **YOU SHOULD NOT JUST** focus in creating new tests, but improving the current ones by removing redundant, flack and not necessary cases
- **YOU SHOULD ALWAYS** read first the @.cursor/rules/test-standards.mdc before start
- **YOU SHOULD NEVER* create a new _test.go file just to add a few tests if a relevant _test.go file already exists.** Instead, always add new or improved tests to the existing _test.go file for that functionality.
</critical>

You are tasked with improving test coverage and quality for the Compozy project in directory: <directory>

## 🎯 PRIMARY OBJECTIVES

1. **Achieve 60% test coverage** for business logic packages in <directory>
2. **Remove unnecessary/redundant/flaky tests** following @.cursor/rules/test-standards.mdc anti-patterns
3. **Reduce over-mocking** - use testify/mock ONLY for external dependencies
4. **Move integration tests** from <directory> to test/integration/ if they exist outside integration directory
5. **Use test helpers** from test/helpers/ for all integration tests

## 📋 STRICT REQUIREMENTS

### ✅ REQUIRED PATTERNS (MANDATORY)

- **Use `t.Run("Should describe behavior")` pattern for ALL tests**
- **Use `stretchr/testify` assertions**: `assert.Equal(t, expected, actual)`, `require.NoError(t, err)`
- **Use testify/mock for external dependencies only**
- **Integration tests MUST be in `test/integration/` directory**
- **Achieve ≥60% coverage on business logic packages**
- **NEVER use testify suite patterns** - use direct functions with `*testing.T`

### ❌ ANTI-PATTERNS TO ELIMINATE (MANDATORY)

**REDUNDANT TESTS:**

- Cross-package validation duplication (same validation logic in multiple packages)
- Identical mock setup patterns repeated across files
- Constructor tests that only verify non-nil objects
- Getter/setter tests without business logic

**LOW-VALUE TESTS:**

- Testing Go standard library functionality
- Testing obvious field assignments
- Testing interface implementations without custom logic
- Mock-heavy tests (90% setup, 10% logic)

**PROHIBITED PATTERNS:**

- `suite.Suite` embedding or any suite-based structures
- Suite methods like `s.Equal()`, `s.NoError()`, `s.T()`
- Weak assertions like `assert.Error(t, err)` without specific validation
- Duplicate integration/unit test coverage

## 🔧 IMPLEMENTATION STEPS

### Step 1: Analyze Current State

1. **Check current test coverage** in <directory> using `go test -cover ./<directory>`
2. **Identify redundant tests** by searching for duplicate validation patterns
3. **Find misplaced integration tests** - any integration tests in <directory> should be moved
4. **Review mock usage** - eliminate over-mocking patterns

### Step 2: Restructure Tests

1. **Move integration tests** from <directory> to `test/integration/<directory>/`
2. **Consolidate duplicate tests** - merge identical validation logic
3. **Remove low-value tests** - eliminate getter/setter, constructor-only tests
4. **Fix test patterns** - convert suite-based tests to t.Run patterns

### Step 3: Improve Test Quality

1. **Use shared helpers** from `test/helpers/`:
   - `GetSharedPostgresTx()` for database integration tests
   - `NewMockSetup()` for database mocking
   - `GenerateUniqueTestID()` for test isolation
   - `StringPtr()`, `IDPtr()` for test data creation

2. **Implement proper mocking**:

   ```go
   type MockService struct {
       mock.Mock
   }

   func (m *MockService) DoSomething(ctx context.Context, param string) error {
       args := m.Called(ctx, param)
       return args.Error(0)
   }
   ```

3. **Add meaningful business logic tests**:

   ```go
   func TestService_Method(t *testing.T) {
       t.Run("Should execute workflow and transition state correctly", func(t *testing.T) {
           // Test actual business logic and state transitions
           workflow := &Workflow{State: StateRunning}
           result, err := workflow.Execute(validInput)

           // Specific assertions about behavior
           require.NoError(t, err)
           assert.Equal(t, StateCompleted, workflow.State)
           assert.Equal(t, expectedOutput, result.Output)
           assert.True(t, workflow.CompletedAt.After(workflow.StartedAt))
       })
   }
   ```

### Step 4: Enhance Coverage

1. **Identify uncovered business logic** in <directory>
2. **Add tests for critical paths**: workflows, state management, error handling
3. **Test error scenarios** and recovery mechanisms
4. **Verify integration points** between components

### Step 5: Validate and Document

1. **Run `make test`** to ensure all tests pass
2. **Run `make lint`** to verify code quality
3. **Check coverage** reaches 60% target
4. **Document test coverage gaps** for future improvement

## 🎯 SUCCESS CRITERIA

### Coverage Targets

- **Business logic packages**: ≥60% coverage
- **All exported functions**: Must have meaningful tests
- **Critical paths**: Must be fully tested
- **Error scenarios**: Must be covered

### Quality Metrics

- **Zero redundant tests** in <directory>
- **All integration tests** moved to `test/integration/`
- **Proper use of helpers** from `test/helpers/`
- **No suite patterns** - all tests use `t.Run()`
- **Specific error validation** - no weak `assert.Error()` calls

### Standards Compliance

- **Follows @.cursor/rules/test-standards.mdc** completely
- **Uses testify correctly** for assertions and mocking
- **Proper test organization** and naming conventions
- **Passes all linters** and CI checks

## 📝 DELIVERABLES

1. **Coverage Report**: Current vs target coverage for <directory>
2. **Moved Tests**: List of integration tests moved to `test/integration/`
3. **Removed Tests**: List of redundant/low-value tests eliminated
4. **Added Tests**: List of new tests added for coverage improvement
5. **Helper Usage**: Documentation of test helpers utilized
6. **Compliance Report**: Verification of standards compliance

## 🚨 CRITICAL VALIDATION

**BEFORE COMPLETING TASK:**

- ✅ `make test` passes all tests
- ✅ `make lint` passes without errors
- ✅ Coverage ≥60% for business logic
- ✅ No integration tests remain in <directory>
- ✅ All tests follow `t.Run("Should...")` pattern
- ✅ No testify suite patterns used
- ✅ Proper use of `test/helpers/` utilities
- ✅ Specific error assertions used throughout

**Failure to meet these criteria will result in task rejection.**

<critical>
- **YOU SHOULD NOT JUST** focus in creating new tests, but improving the current ones by removing redundant, flack and not necessary cases
- **YOU SHOULD ALWAYS** read first the @.cursor/rules/test-standards.mdc before start
- **YOU SHOULD NEVER* create a new _test.go file just to add a few tests if a relevant _test.go file already exists.** Instead, always add new or improved tests to the existing _test.go file for that functionality.
</critical>
