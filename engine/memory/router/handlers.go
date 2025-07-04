package memrouter

import (
	"net/http"
	"strconv"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/memory/service"
	memuc "github.com/compozy/compozy/engine/memory/uc"
	"github.com/gin-gonic/gin"
)

// -----
// Read Memory
// -----

// readMemory retrieves memory content
//
//	@Summary		Read memory content
//	@Description	Retrieve memory content for a specific memory reference and key
//	@Tags			memory
//	@Accept			json
//	@Produce		json
//	@Param			memory_ref	path		string	true	"Memory reference"	example("user_memory")
//	@Param			key			query		string	true	"Memory key"		example("user:123:profile")
//	@Param			limit		query		int		false	"Maximum number of messages to return (default: 50, max: 1000)"	example(50)
//	@Param			offset		query		int		false	"Number of messages to skip (for pagination)"	example(0)
//	@Success		200			{object}	router.Response{data=object{messages=[]object{role=string,content=string},total_count=int,has_more=bool,key=string,limit=int,offset=int}}	"Memory read successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		404			{object}	router.Response{error=router.ErrorInfo}	"Memory not found"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/api/v0/memory/{memory_ref}/read [get]
func readMemory(c *gin.Context) {
	// Get memory context from middleware
	memCtx, ok := GetMemoryContext(c)
	if !ok {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"memory context not found",
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Parse pagination parameters
	limit := 50 // default
	offset := 0 // default

	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
			// Enforce maximum limit
			limit = min(limit, 1000)
			if limit <= 0 {
				limit = 50
			}
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil {
			offset = max(parsedOffset, 0)
		}
	}

	// Execute use case
	uc := memuc.NewReadMemory(memCtx.Manager, memCtx.Worker, nil)

	input := memuc.ReadMemoryInput{
		MemoryRef: memCtx.MemoryRef,
		Key:       memCtx.Key,
		Limit:     limit,
		Offset:    offset,
	}

	output, err := uc.Execute(c.Request.Context(), input)
	if err != nil {
		handleMemoryError(c, err, "failed to read memory")
		return
	}

	// Convert messages to response format
	result := map[string]any{
		"key":         memCtx.Key,
		"messages":    convertMessagesToResponse(output.Messages),
		"total_count": output.TotalCount,
		"has_more":    output.HasMore,
		"limit":       limit,
		"offset":      offset,
	}

	router.RespondOK(c, "memory read successfully", result)
}

// -----
// Write Memory
// -----

// writeMemory writes/replaces memory content
//
//	@Summary		Write memory content
//	@Description	Write or replace memory content for a specific memory reference and key
//	@Tags			memory
//	@Accept			json
//	@Produce		json
//	@Param			memory_ref	path		string	true	"Memory reference"	example("user_memory")
//	@Param			body		body		WriteMemoryRequest	true	"Key and messages to write"
//	@Success		200			{object}	router.Response{data=service.WriteResponse}	"Memory written successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/api/v0/memory/{memory_ref}/write [post]
func writeMemory(c *gin.Context) {
	// Get memory context from middleware
	memCtx, ok := GetMemoryContext(c)
	if !ok {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"memory context not found",
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Parse request body with key
	var req WriteMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			"invalid request body",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Create input for use case
	input := memuc.WriteMemoryInput{
		Messages: req.Messages,
	}

	// Execute use case
	memService := service.NewMemoryOperationsService(memCtx.Manager, nil, nil)
	uc := memuc.NewWriteMemory(memService, memCtx.MemoryRef, req.Key, &input)
	result, err := uc.Execute(c.Request.Context())
	if err != nil {
		handleMemoryError(c, err, "failed to write memory")
		return
	}

	router.RespondOK(c, "memory written successfully", result)
}

// -----
// Append Memory
// -----

// appendMemory appends to memory content
//
//	@Summary		Append to memory
//	@Description	Append messages to existing memory content
//	@Tags			memory
//	@Accept			json
//	@Produce		json
//	@Param			memory_ref	path		string	true	"Memory reference"	example("user_memory")
//	@Param			body		body		AppendMemoryRequest	true	"Key and messages to append"
//	@Success		200			{object}	router.Response{data=service.AppendResponse}	"Memory appended successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/api/v0/memory/{memory_ref}/append [post]
func appendMemory(c *gin.Context) {
	// Get memory context from middleware
	memCtx, ok := GetMemoryContext(c)
	if !ok {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"memory context not found",
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Parse request body with key
	var req AppendMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			"invalid request body",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Create input for use case
	input := memuc.AppendMemoryInput{
		Messages: req.Messages,
	}

	// Execute use case
	uc := memuc.NewAppendMemory(memCtx.Manager, memCtx.MemoryRef, req.Key, &input, nil)
	result, err := uc.Execute(c.Request.Context())
	if err != nil {
		handleMemoryError(c, err, "failed to append to memory")
		return
	}

	router.RespondOK(c, "memory appended successfully", result)
}

// -----
// Delete Memory
// -----

// deleteMemory deletes memory content
//
//	@Summary		Delete memory
//	@Description	Delete all memory content for a specific key
//	@Tags			memory
//	@Accept			json
//	@Produce		json
//	@Param			memory_ref	path		string	true	"Memory reference"	example("user_memory")
//	@Param			body		body		DeleteMemoryRequest	true	"Key to delete"
//	@Success		200			{object}	router.Response{data=service.DeleteResponse}	"Memory deleted successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/api/v0/memory/{memory_ref}/delete [post]
func deleteMemory(c *gin.Context) {
	// Get memory context from middleware
	memCtx, ok := GetMemoryContext(c)
	if !ok {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"memory context not found",
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Parse request body with key
	var req DeleteMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			"invalid request body",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Execute use case
	uc := memuc.NewDeleteMemory(memCtx.Manager, memCtx.MemoryRef, req.Key, nil)
	result, err := uc.Execute(c.Request.Context())
	if err != nil {
		handleMemoryError(c, err, "failed to delete memory")
		return
	}

	router.RespondOK(c, "memory deleted successfully", result)
}

// -----
// Flush Memory
// -----

// flushMemory flushes memory content
//
//	@Summary		Flush memory
//	@Description	Flush memory content with optional summarization. The actual_strategy field in the response indicates which flush strategy was used.
//	@Tags			memory
//	@Accept			json
//	@Produce		json
//	@Param			memory_ref	path		string	true	"Memory reference"	example("user_memory")
//	@Param			body		body		FlushMemoryRequest	true	"Key and flush options"
//	@Success		200			{object}	router.Response{data=memuc.FlushMemoryResult}	"Memory flushed successfully. Response includes actual_strategy field showing which strategy was used"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/api/v0/memory/{memory_ref}/flush [post]
func flushMemory(c *gin.Context) {
	// Get memory context from middleware
	memCtx, ok := GetMemoryContext(c)
	if !ok {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"memory context not found",
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Parse request body with key
	var req FlushMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			"invalid request body",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Create input for use case
	input := memuc.FlushMemoryInput{
		Force:    req.Force,
		DryRun:   req.DryRun,
		MaxKeys:  req.MaxKeys,
		Strategy: req.Strategy,
	}

	// Execute use case
	uc := memuc.NewFlushMemory(memCtx.Manager, memCtx.MemoryRef, req.Key, &input, nil)
	result, err := uc.Execute(c.Request.Context())
	if err != nil {
		handleMemoryError(c, err, "failed to flush memory")
		return
	}

	router.RespondOK(c, "memory flushed successfully", result)
}

// -----
// Memory Health
// -----

// healthMemory checks memory health
//
//	@Summary		Check memory health
//	@Description	Get health status and metrics for memory
//	@Tags			memory
//	@Accept			json
//	@Produce		json
//	@Param			memory_ref		path		string	true	"Memory reference"		example("user_memory")
//	@Param			key				query		string	true	"Memory key"			example("user:123:profile")
//	@Param			include_stats	query		bool	false	"Include detailed stats"	example(true)
//	@Success		200				{object}	router.Response{data=memuc.HealthMemoryResult}	"Memory health retrieved successfully"
//	@Failure		400				{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		500				{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/api/v0/memory/{memory_ref}/health [get]
func healthMemory(c *gin.Context) {
	// Get memory context from middleware
	memCtx, ok := GetMemoryContext(c)
	if !ok {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"memory context not found",
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Get query parameter
	includeStats := c.Query("include_stats") == "true"

	// Execute use case
	input := &memuc.HealthMemoryInput{
		IncludeStats: includeStats,
	}
	uc := memuc.NewHealthMemory(memCtx.Manager, memCtx.MemoryRef, memCtx.Key, input, nil)
	result, err := uc.Execute(c.Request.Context())
	if err != nil {
		handleMemoryError(c, err, "failed to get memory health")
		return
	}

	router.RespondOK(c, "memory health retrieved successfully", result)
}

// -----
// Clear Memory
// -----

// clearMemory clears all memory content
//
//	@Summary		Clear memory
//	@Description	Clear all memory content with confirmation
//	@Tags			memory
//	@Accept			json
//	@Produce		json
//	@Param			memory_ref	path		string	true	"Memory reference"	example("user_memory")
//	@Param			body		body		ClearMemoryRequest	true	"Key and clear options"
//	@Success		200			{object}	router.Response{data=memuc.ClearMemoryResult}	"Memory cleared successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/api/v0/memory/{memory_ref}/clear [post]
func clearMemory(c *gin.Context) {
	// Get memory context from middleware
	memCtx, ok := GetMemoryContext(c)
	if !ok {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"memory context not found",
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Parse request body with key
	var req ClearMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			"invalid request body",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Create input for use case
	input := memuc.ClearMemoryInput{
		Confirm: req.Confirm,
		Backup:  req.Backup,
	}

	// Execute use case
	uc := memuc.NewClearMemory(memCtx.Manager, memCtx.MemoryRef, req.Key, &input, nil)
	result, err := uc.Execute(c.Request.Context())
	if err != nil {
		handleMemoryError(c, err, "failed to clear memory")
		return
	}

	router.RespondOK(c, "memory cleared successfully", result)
}

// -----
// Memory Stats
// -----

// statsMemory gets memory statistics
//
//	@Summary		Get memory statistics
//	@Description	Retrieve detailed statistics about memory content
//	@Tags			memory
//	@Accept			json
//	@Produce		json
//	@Param			memory_ref	path		string	true	"Memory reference"	example("user_memory")
//	@Param			key			query		string	true	"Memory key"		example("user:123:profile")
//	@Param			limit		query		int		false	"Limit for role distribution calculation (default: 100, max: 10000)"	example(100)
//	@Param			offset		query		int		false	"Offset for role distribution calculation"	example(0)
//	@Success		200			{object}	router.Response{data=memuc.StatsMemoryOutput}	"Memory statistics retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/api/v0/memory/{memory_ref}/stats [get]
func statsMemory(c *gin.Context) {
	// Get memory context from middleware
	memCtx, ok := GetMemoryContext(c)
	if !ok {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"memory context not found",
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Parse pagination parameters for role distribution
	limit := 100 // default for stats
	offset := 0  // default

	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsedOffset
		}
	}

	// Create use case
	uc := memuc.NewStatsMemory(memCtx.Manager, memCtx.Worker, nil)

	// Execute
	result, err := uc.Execute(c.Request.Context(), memuc.StatsMemoryInput{
		MemoryRef: memCtx.MemoryRef,
		Key:       memCtx.Key,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		handleMemoryError(c, err, "failed to get memory statistics")
		return
	}

	router.RespondOK(c, "memory statistics retrieved successfully", result)
}
