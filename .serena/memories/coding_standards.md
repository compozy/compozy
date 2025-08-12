# Compozy Coding Standards and Conventions

## Code Style and Conventions

### Go Coding Standards

- **Functions**: Limit function complexity and size
- **Error Handling**: Use Go idioms for error handling
- **Documentation**: Follow Go documentation standards
- **Testing**: Use testify framework with `t.Run("Should...")` pattern
- **Architecture**: Follow SOLID principles and Clean Architecture
- **Libraries**: Use established core libraries as specified in project rules

### Code Formatting and Line Spacing

- **CRITICAL**: NEVER add blank lines inside function bodies
- Use `gofmt` and `golangci-lint` for formatting
- Follow established patterns in the codebase

### Testing Standards

- **MANDATORY**: Use `t.Run("Should...")` pattern for all tests
- Use testify assertions and mocks
- Mock external dependencies when necessary
- Test coverage should be comprehensive

### Architecture Principles

- **SOLID Principles**: Single responsibility, Open/closed, Liskov substitution, Interface segregation, Dependency inversion
- **Clean Architecture**: Separate concerns into layers
- **DRY**: Don't repeat yourself
- Constructor dependency injection pattern
- Context-first approach: `logger.FromContext(ctx)`
- Interface boundaries between layers

### API Development

- RESTful design principles
- Proper versioning strategy
- Comprehensive documentation
- Security requirements compliance

### Code Quality and Security

- Follow linting rules strictly
- Implement security requirements
- Use mandatory library choices consistently

### Go Implementation Patterns

- Canonical implementations of architecture principles
- Follow established patterns in the codebase
- Use proper interface designs
- Implement proper error handling patterns
