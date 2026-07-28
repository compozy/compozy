# Architectural Analysis Report
**Date**: [timestamp]
**Files Analyzed**: X
**Dead Code Files**: Y
**Duplication Groups**: Z

---

## Executive Summary
- **Dead Code**: X files, Y exports completely unused
- **Duplicated Functionality**: Z duplication groups
- **Architectural Anti-Patterns**: W issues
- **Type Issues**: V problematic usages
- **Code Smells**: U instances

**Estimated Cleanup**: Remove ~X lines of dead code, consolidate Y duplications

---

## Dead Code

### Completely Dead Files (DELETE)
| File | Reason | Confidence |
|------|--------|------------|
| `src/old/legacy-processor.ts` | No imports found | HIGH |
| `src/utils/unused-helper.ts` | Exported but never used | HIGH |
| `src/temp/temp-service.ts` | Temporary file left behind | HIGH |

**Total Lines**: X,XXX lines can be deleted

### Dead Exports (REMOVE)
| File | Export | Reason |
|------|--------|--------|
| `src/utils/format.ts` | `formatOldDate()` | Replaced by `formatDate()`, no usage |
| `src/services/auth.ts` | `oldLogin()` | Deprecated, no usage found |

### Possibly Dead (VERIFY)
| File | Export | Reason | Verification Needed |
|------|--------|--------|---------------------|
| `src/lib/api.ts` | `fetchOldApi()` | Only used in commented code | Check if truly deprecated |

### Internal Dead Code
- `src/services/user.ts:125` - Private method `_validateLegacy()` never called
- `src/components/form.tsx:89` - Variable `tempData` assigned but never read

---

## Duplicated Functionality

### CRITICAL: Exact Duplicates

#### Duplication Group 1: Email Validation
**Instances**: 3
**Files**:
- `src/utils/validators.ts:42` - `validateEmail(email: string)`
- `src/lib/email.ts:15` - `isValidEmail(email: string)`
- `src/components/forms/validation.ts:67` - `checkEmailFormat(email: string)`

**Analysis**: All three use identical regex pattern `/^[^\s@]+@[^\s@]+\.[^\s@]+$/`
**Lines Duplicated**: ~15 lines × 3 = 45 lines
**Recommendation**:
- Keep: `src/utils/validators.ts:validateEmail()`
- Remove: Other two implementations
- Update: All imports to use validators version

#### Duplication Group 2: API Error Handling
**Instances**: 4
**Files**: [list]
**Analysis**: [similar]

### HIGH: Similar Logic

#### Duplication Group: Date Formatting
**Instances**: 2
**Files**:
- `src/utils/date.ts:30` - `formatDate()` - Uses date-fns
- `src/lib/format.ts:45` - `formatDateTime()` - Uses native Date

**Analysis**: Both format dates but use different libraries
**Recommendation**: Standardize on date-fns, remove native version

### Type Duplication

#### Type Group: User Interface
**Instances**: 3
**Files**:
- `src/types/user.ts` - `User` interface
- `src/models/user.ts` - `UserModel` interface (identical fields)
- `src/api/types.ts` - `UserData` interface (identical fields)

**Recommendation**: Use single `User` type from `src/types/user.ts`

---

## Architectural Anti-Patterns

### God Objects

#### `src/services/application-manager.ts` (850 lines)
**Responsibilities**: Database, auth, config, logging, caching, validation
**Issue**: Violates SRP, does everything
**Recommendation**: Split into:
- `database.service.ts`
- `auth.service.ts`
- `config.service.ts`
- `logging.service.ts`

### Circular Dependencies

#### Cycle 1: `auth.ts` ↔ `user.ts`
- `auth.ts` imports `getUserById` from `user.ts`
- `user.ts` imports `validateToken` from `auth.ts`
**Issue**: Creates tight coupling, makes testing hard
**Recommendation**: Extract shared types to separate file

### Tight Coupling

#### `components/UserForm.tsx` → `services/database.ts`
**Issue**: UI component directly importing database layer
**Recommendation**: Use service layer abstraction

### Layer Violations

#### `models/User.ts` imports from `components/`
**Issue**: Model layer should not know about view layer
**Recommendation**: Remove dependency, pass data via props

---

## Type Issues

### `any` Usage (X instances)

| File | Line | Context | Severity |
|------|------|---------|----------|
| `src/api/client.ts` | 45 | `response: any` | HIGH |
| `src/utils/parse.ts` | 23 | `data: any` | HIGH |

**Total `any` usages**: X
**Recommendation**: Define proper types for all cases

### Type Assertions (Y instances)

| File | Line | Assertion | Issue |
|------|------|-----------|-------|
| `src/lib/api.ts` | 67 | `as User` | Unsafe cast, no validation |
| `src/utils/parse.ts` | 89 | `as unknown as T` | Double cast to bypass types |

**Issue**: Type safety bypassed, runtime errors possible

### @ts-ignore Comments (Z instances)

| File | Line | Reason | Should Fix |
|------|------|--------|------------|
| `src/legacy/old.ts` | 34 | "Type error in legacy code" | Refactor or remove file |

---

## Code Smells

### Long Functions (>50 lines)

| File | Function | Lines | Issue |
|------|----------|-------|-------|
| `src/services/processor.ts` | `processData()` | 127 | Does too much, hard to test |

**Recommendation**: Extract smaller functions

### Complex Conditionals

| File | Line | Issue |
|------|------|-------|
| `src/utils/validator.ts` | 45 | Nested 4 levels deep |
| `src/lib/parser.ts` | 89 | Boolean expression spans 3 lines |

### Magic Numbers

| File | Line | Magic Value | Should Be |
|------|------|-------------|-----------|
| `src/config/limits.ts` | 12 | `86400` | `SECONDS_PER_DAY` |
| `src/utils/format.ts` | 34 | `1000` | `MS_PER_SECOND` |

### Commented-Out Code

**Files with commented code**: X
- `src/old/legacy.ts` - 45 lines of commented code
- `src/services/auth.ts` - Old implementation commented out

**Recommendation**: Delete all commented code (use git history)

---

## Statistics

**Dead Code**:
- Files: X
- Exports: Y
- Lines: Z (estimated)

**Duplication**:
- Groups: X
- Files affected: Y
- Duplicated lines: ~Z

**Architectural Issues**:
- God objects: X
- Circular dependencies: Y
- Layer violations: Z

**Type Issues**:
- `any` usage: X
- Type assertions: Y
- @ts-ignore: Z

**Code Smells**:
- Long functions: X
- Complex conditionals: Y
- Magic numbers: Z

---

## Impact Assessment

### Code Cleanup Potential
- **Dead code removal**: ~X,XXX lines
- **Duplication consolidation**: ~Y,YYY lines
- **Total reduction**: ~Z,ZZZ lines (AA% of codebase)

### Maintainability Improvement
- Fewer places to update when fixing bugs
- Clearer code responsibilities
- Better type safety
- Reduced cognitive load

### Risk Areas
- High coupling in `services/` directory
- Type safety compromised in `api/` layer
- Architectural violations in `components/`
