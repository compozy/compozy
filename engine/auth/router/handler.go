package router

import (
	"net/http"
	"time"

	"github.com/compozy/compozy/engine/auth"
	"github.com/compozy/compozy/engine/auth/model"
	"github.com/compozy/compozy/engine/auth/uc"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// Handler handles auth-related HTTP requests
type Handler struct {
	factory *uc.Factory
}

// NewHandler creates a new auth handler
func NewHandler(factory *uc.Factory) *Handler {
	return &Handler{
		factory: factory,
	}
}

// getUserIDFromContext extracts and parses the user ID from the gin context
func (h *Handler) getUserIDFromContext(c *gin.Context) (core.ID, bool) {
	log := logger.FromContext(c.Request.Context())
	userIDStr, exists := c.Get(auth.ContextKeyUserID)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return "", false
	}
	userIDString, ok := userIDStr.(string)
	if !ok {
		log.Error("User ID is not a string", "user_id", userIDStr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return "", false
	}
	userID, parseErr := core.ParseID(userIDString)
	if parseErr != nil {
		log.Error("Invalid user ID in context", "error", parseErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return "", false
	}
	return userID, true
}

// GenerateKey godoc
// @Summary Generate a new API key
// @Description Generate a new API key for the authenticated user
// @Tags auth
// @Accept json
// @Produce json
// @Success 201 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /auth/generate [post]
func (h *Handler) GenerateKey(c *gin.Context) {
	ctx := c.Request.Context()
	log := logger.FromContext(ctx)
	// Get user ID from context (set by auth middleware)
	userID, ok := h.getUserIDFromContext(c)
	if !ok {
		return // Error response already sent by helper
	}
	// Generate the API key
	generateUC := h.factory.GenerateAPIKey(userID)
	apiKey, err := generateUC.Execute(ctx)
	if err != nil {
		log.Error("Failed to generate API key", "error", err, "user_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate API key"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"api_key": apiKey,
		"message": "API key generated successfully. Please save it securely as it cannot be retrieved again.",
	})
}

// ListKeys godoc
// @Summary List user's API keys
// @Description List all API keys for the authenticated user
// @Tags auth
// @Accept json
// @Produce json
// @Success 200 {array} model.APIKey
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /auth/keys [get]
func (h *Handler) ListKeys(c *gin.Context) {
	ctx := c.Request.Context()
	log := logger.FromContext(ctx)
	// Get user ID from context
	userID, ok := h.getUserIDFromContext(c)
	if !ok {
		return // Error response already sent by helper
	}
	// List API keys
	listUC := h.factory.ListAPIKeys(userID)
	keys, err := listUC.Execute(ctx)
	if err != nil {
		log.Error("Failed to list API keys", "error", err, "user_id", userID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list API keys"})
		return
	}
	// Mask the hash field for security
	maskedKeys := make([]APIKeyMetadataResponse, len(keys))
	for i, key := range keys {
		metadata := APIKeyMetadataResponse{
			ID:        key.ID.String(),
			Prefix:    key.Prefix,
			CreatedAt: key.CreatedAt,
		}
		// Handle nullable LastUsed
		if key.LastUsed.Valid {
			metadata.LastUsed = &key.LastUsed.Time
		}
		maskedKeys[i] = metadata
	}
	c.JSON(http.StatusOK, APIKeysListResponse{Keys: maskedKeys})
}

// RevokeKey godoc
// @Summary Revoke an API key
// @Description Revoke an API key by ID
// @Tags auth
// @Accept json
// @Produce json
// @Param id path string true "API Key ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /auth/keys/{id} [delete]
func (h *Handler) RevokeKey(c *gin.Context) {
	ctx := c.Request.Context()
	log := logger.FromContext(ctx)
	// Get user ID from context
	userID, ok := h.getUserIDFromContext(c)
	if !ok {
		return // Error response already sent by helper
	}
	// Get key ID from URL
	keyIDStr := c.Param("id")
	keyID, err := core.ParseID(keyIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid key ID"})
		return
	}
	// Revoke the key (authorization check is now in the use case)
	revokeUC := h.factory.RevokeAPIKey(userID, keyID)
	err = revokeUC.Execute(ctx)
	if err != nil {
		log.Error("Failed to revoke API key", "error", err, "key_id", keyID)
		// Handle specific error types
		if coreErr, ok := err.(*core.Error); ok {
			switch coreErr.Code {
			case auth.ErrCodeNotFound:
				c.JSON(http.StatusNotFound, gin.H{"error": "API key not found"})
				return
			case auth.ErrCodeForbidden:
				c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke API key"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "API key revoked successfully"})
}

// CreateUser godoc
// @Summary Create a new user (admin only)
// @Description Create a new user with the specified email and role
// @Tags users
// @Accept json
// @Produce json
// @Param user body CreateUserRequest true "User details"
// @Success 201 {object} model.User
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /users [post]
func (h *Handler) CreateUser(c *gin.Context) {
	ctx := c.Request.Context()
	log := logger.FromContext(ctx)
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	// Validate role
	role := model.RoleUser // default
	if req.Role != "" {
		switch req.Role {
		case string(model.RoleAdmin), string(model.RoleUser):
			role = model.Role(req.Role)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role"})
			return
		}
	}
	// Create user through use case
	createUC := h.factory.CreateUser(&uc.CreateUserInput{
		Email: req.Email,
		Role:  role,
	})
	user, err := createUC.Execute(ctx)
	if err != nil {
		log.Error("Failed to create user", "error", err)
		// Handle specific error types
		if coreErr, ok := err.(*core.Error); ok {
			switch coreErr.Code {
			case auth.ErrCodeInvalidEmail, auth.ErrCodeWeakPassword, auth.ErrCodeInvalidRole:
				c.JSON(http.StatusBadRequest, gin.H{"error": coreErr.Message})
				return
			case auth.ErrCodeEmailExists:
				c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}
	c.JSON(http.StatusCreated, user)
}

// ListUsers godoc
// @Summary List all users (admin only)
// @Description List all users in the system
// @Tags users
// @Accept json
// @Produce json
// @Success 200 {array} model.User
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /users [get]
func (h *Handler) ListUsers(c *gin.Context) {
	ctx := c.Request.Context()
	log := logger.FromContext(ctx)
	// List users through use case
	listUC := h.factory.ListUsers()
	users, err := listUC.Execute(ctx)
	if err != nil {
		log.Error("Failed to list users", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

// UpdateUser godoc
// @Summary Update a user (admin only)
// @Description Update a user's email or role
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param user body UpdateUserRequest true "User update details"
// @Success 200 {object} model.User
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /users/{id} [patch]
func (h *Handler) UpdateUser(c *gin.Context) {
	ctx := c.Request.Context()
	log := logger.FromContext(ctx)
	// Get user ID from URL
	userIDStr := c.Param("id")
	userID, err := core.ParseID(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}
	// Validate role if provided
	var rolePtr *model.Role
	if req.Role != nil {
		switch *req.Role {
		case string(model.RoleAdmin), string(model.RoleUser):
			role := model.Role(*req.Role)
			rolePtr = &role
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid role"})
			return
		}
	}
	// Update user through use case
	updateUC := h.factory.UpdateUser(userID, &uc.UpdateUserInput{
		Email: req.Email,
		Role:  rolePtr,
	})
	user, err := updateUC.Execute(ctx)
	if err != nil {
		log.Error("Failed to update user", "error", err, "user_id", userID)
		// Handle specific error types
		if coreErr, ok := err.(*core.Error); ok {
			switch coreErr.Code {
			case auth.ErrCodeNotFound:
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			case auth.ErrCodeEmailExists:
				c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// DeleteUser godoc
// @Summary Delete a user (admin only)
// @Description Delete a user by ID
// @Tags users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /users/{id} [delete]
func (h *Handler) DeleteUser(c *gin.Context) {
	ctx := c.Request.Context()
	log := logger.FromContext(ctx)
	// Get user ID from URL
	userIDStr := c.Param("id")
	userID, err := core.ParseID(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}
	// Delete user through use case
	deleteUC := h.factory.DeleteUser(userID)
	err = deleteUC.Execute(ctx)
	if err != nil {
		log.Error("Failed to delete user", "error", err, "user_id", userID)
		// Handle specific error types
		if coreErr, ok := err.(*core.Error); ok {
			if coreErr.Code == auth.ErrCodeNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			}
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// CreateUserRequest represents the request to create a user
type CreateUserRequest struct {
	Email string `json:"email" binding:"required"`
	Role  string `json:"role"`
}

// UpdateUserRequest represents the request to update a user
type UpdateUserRequest struct {
	Email *string `json:"email,omitempty"`
	Role  *string `json:"role,omitempty"`
}

// APIKeyMetadataResponse represents the response for API key metadata
type APIKeyMetadataResponse struct {
	ID        string     `json:"id"`
	Prefix    string     `json:"prefix"`
	CreatedAt time.Time  `json:"created_at"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
}

// APIKeysListResponse represents the response for listing API keys
type APIKeysListResponse struct {
	Keys []APIKeyMetadataResponse `json:"keys"`
}
