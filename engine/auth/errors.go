package auth

// Error codes used throughout the auth domain
const (
	// ErrCodeNotFound indicates a resource was not found
	ErrCodeNotFound = "NOT_FOUND"
	// ErrCodeForbidden indicates access is denied
	ErrCodeForbidden = "FORBIDDEN"
	// ErrCodeEmailExists indicates email already exists
	ErrCodeEmailExists = "EMAIL_EXISTS"
	// ErrCodeInvalidEmail indicates invalid email format
	ErrCodeInvalidEmail = "INVALID_EMAIL"
	// ErrCodeWeakPassword indicates password doesn't meet requirements
	ErrCodeWeakPassword = "WEAK_PASSWORD"
	// ErrCodeInvalidRole indicates invalid role specified
	ErrCodeInvalidRole = "INVALID_ROLE"
)

// Context keys used for auth middleware
const (
	// ContextKeyAPIKey is the context key for API key
	ContextKeyAPIKey = "apiKey"
	// ContextKeyUserID is the context key for user ID
	ContextKeyUserID = "userID"
	// ContextKeyUserRole is the context key for user role
	ContextKeyUserRole = "userRole"
)
