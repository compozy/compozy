# Resolution Patterns & Heuristics

## Pattern 1: Main Added New Validation/Security

**Indicator**: Main's version has `if` checks or validations you don't have

**Action**: Keep main's validation, integrate with your feature

```javascript
// WRONG: Removing main's validation
function authenticate(token) {
  validateToken(token); // Lost: if (!token) check
  return true;
}

// RIGHT: Keeping all validations
function authenticate(token) {
  if (!token) throw new Error("No token"); // main's addition
  validateToken(token); // your code
  return true;
}
```

## Pattern 2: Your Feature Added Critical Functionality

**Indicator**: Main's version doesn't have your key feature additions

**Action**: Ensure your additions are preserved

```typescript
// Before conflict
export class AuthService {
  validate() {}
  // Your addition
  setUserContext(user) {}
}

// After resolving (keep your addition)
export class AuthService {
  validate() {} // main's version
  setUserContext(user) {} // your addition (don't lose this!)
}
```

## Pattern 3: Both Changed Same Line (Logic Conflict)

**Indicator**: Same line different in both versions

**Action**: Understand why, choose based on correctness

```javascript
// YOU: Changed return value
return new User(profile);

// MAIN: Changed return statement entirely
return User.fromProfile(profile);

// DECISION: What's the actual implementation?
// Check User class for which method exists
// Pick the one that matches current API
```

## Pattern 4: Main Deleted Code You Still Need

**Indicator**: Code block exists in your branch, missing in main

**Action**: Keep your code - main might have deleted prematurely

```javascript
// Main deleted some logging
// You kept it in your feature
// DECISION: If logging is useful, keep it
// If it was debug code, you can delete it too

// Usually: Keep your feature code + main's latest
```
