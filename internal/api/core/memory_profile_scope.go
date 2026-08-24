package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/memory"
	profilepkg "github.com/compozy/compozy/internal/profile"
	storepkg "github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

type memoryProfileBinding struct {
	ID   string
	Name string
}

type memoryProfileBindingContextKey struct{}

// BindMemoryProfile resolves one profile owner for every public memory request.
func (h *BaseHandlers) BindMemoryProfile(c *gin.Context) {
	scope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		c.Abort()
		return
	}
	if scope.AllProfiles {
		err := &profilepkg.Error{
			Code:    profileSelectionConflictCode,
			Message: "memory reads require one profile",
			Action:  "choose a profile instead of all profiles",
			Cause:   profilepkg.ErrInvalidInput,
		}
		h.respondProfileReadScopeError(c, err)
		c.Abort()
		return
	}
	name, err := h.agentResourceProfileNameForScope(c.Request.Context(), scope)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		c.Abort()
		return
	}
	binding := memoryProfileBinding{ID: strings.TrimSpace(scope.ProfileID), Name: strings.TrimSpace(name)}
	ctx := context.WithValue(c.Request.Context(), memoryProfileBindingContextKey{}, binding)
	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

func memoryProfileBindingFromContext(ctx context.Context) memoryProfileBinding {
	if ctx != nil {
		if binding, ok := ctx.Value(memoryProfileBindingContextKey{}).(memoryProfileBinding); ok {
			return binding
		}
	}
	return memoryProfileBinding{ID: storepkg.DefaultProfileID, Name: compozyconfig.DefaultProfileDirName}
}

func (h *BaseHandlers) memoryProfileStore(selector memorySelector) (*memory.Store, error) {
	if h == nil || h.MemoryStore == nil {
		return nil, fmt.Errorf("memory store is not configured")
	}
	profileID := strings.TrimSpace(selector.ProfileID)
	profileName := strings.TrimSpace(selector.ProfileName)
	if profileID == "" || profileName == "" {
		return h.MemoryStore, nil
	}
	if profileID == storepkg.DefaultProfileID && profileName == compozyconfig.DefaultProfileDirName {
		return h.MemoryStore, nil
	}
	profilesDir := strings.TrimSpace(h.HomePaths.ProfilesDir)
	if profilesDir == "" {
		return nil, fmt.Errorf("memory: profiles directory is not configured")
	}
	return h.MemoryStore.ForProfile(
		profileID,
		filepath.Join(profilesDir, profileName, compozyconfig.MemoryDirName),
	), nil
}

// memoryBoundStore returns the profile store selected by BindMemoryProfile.
// Memory handlers must use this store for every read and write; the process
// default store is not a valid fallback for a named profile.
func (h *BaseHandlers) memoryBoundStore(ctx context.Context) (*memory.Store, error) {
	binding := memoryProfileBindingFromContext(ctx)
	return h.memoryProfileStore(memorySelector{ProfileID: binding.ID, ProfileName: binding.Name})
}
