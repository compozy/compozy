package memrouter

import (
	"net/http"
	"strconv"

	"github.com/compozy/compozy/engine/infra/server/router"
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
//	@Param			key			path		string	true	"Memory key"		example("user:123:profile")
//	@Param			limit		query		int		false	"Maximum number of messages to return (default: 50, max: 1000)"	example(50)
//	@Param			offset		query		int		false	"Number of messages to skip (for pagination)"	example(0)
//	@Success		200			{object}	router.Response{data=object{messages=[]object{role=string,content=string},total_count=int,has_more=bool,key=string,limit=int,offset=int}}	"Memory read successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		404			{object}	router.Response{error=router.ErrorInfo}	"Memory not found"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/api/v0/memory/{memory_ref}/{key} [get]
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
		}
	}

	if offsetStr := c.Query("offset"); offsetStr != "" {
		if parsedOffset, err := strconv.Atoi(offsetStr); err == nil {
			offset = parsedOffset
		}
	}

	// Execute use case
	uc := &memuc.ReadMemory{
		Manager: memCtx.Manager,
		Worker:  memCtx.Worker,
	}

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
//	@Param			key			path		string	true	"Memory key"		example("user:123:profile")
//	@Param			body		body		memuc.WriteMemoryInput	true	"Messages to write"
//	@Success		200			{object}	router.Response{data=memuc.WriteMemoryResult}	"Memory written successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/api/v0/memory/{memory_ref}/{key} [put]
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

	// Parse request body
	var input memuc.WriteMemoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			"invalid request body",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Execute use case
	uc := memuc.NewWriteMemory(memCtx.Manager, memCtx.MemoryRef, memCtx.Key, &input)
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
//	@Param			key			path		string	true	"Memory key"		example("user:123:profile")
//	@Param			body		body		memuc.AppendMemoryInput	true	"Messages to append"
//	@Success		200			{object}	router.Response{data=memuc.AppendMemoryResult}	"Memory appended successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/api/v0/memory/{memory_ref}/{key} [post]
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

	// Parse request body
	var input memuc.AppendMemoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			"invalid request body",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Execute use case
	uc := memuc.NewAppendMemory(memCtx.Manager, memCtx.MemoryRef, memCtx.Key, &input)
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
//	@Param			key			path		string	true	"Memory key"		example("user:123:profile")
//	@Success		200			{object}	router.Response{data=memuc.DeleteMemoryResult}	"Memory deleted successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/api/v0/memory/{memory_ref}/{key} [delete]
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

	// Execute use case
	uc := memuc.NewDeleteMemory(memCtx.Manager, memCtx.MemoryRef, memCtx.Key)
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
//	@Description	Flush memory content with optional summarization
//	@Tags			memory
//	@Accept			json
//	@Produce		json
//	@Param			memory_ref	path		string	true	"Memory reference"	example("user_memory")
//	@Param			key			path		string	true	"Memory key"		example("user:123:profile")
//	@Param			body		body		memuc.FlushMemoryInput	false	"Flush options"
//	@Success		200			{object}	router.Response{data=memuc.FlushMemoryResult}	"Memory flushed successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/api/v0/memory/{memory_ref}/{key}/flush [post]
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

	// Parse request body (optional)
	var input memuc.FlushMemoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		// Continue with default values
		input = memuc.FlushMemoryInput{}
	}

	// Execute use case
	uc := memuc.NewFlushMemory(memCtx.Manager, memCtx.MemoryRef, memCtx.Key, &input)
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
//	@Param			key				path		string	true	"Memory key"			example("user:123:profile")
//	@Param			include_stats	query		bool	false	"Include detailed stats"	example(true)
//	@Success		200				{object}	router.Response{data=memuc.HealthMemoryResult}	"Memory health retrieved successfully"
//	@Failure		400				{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		500				{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/api/v0/memory/{memory_ref}/{key}/health [get]
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
	uc := memuc.NewHealthMemory(memCtx.Manager, memCtx.MemoryRef, memCtx.Key, input)
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
//	@Param			key			path		string	true	"Memory key"		example("user:123:profile")
//	@Param			body		body		memuc.ClearMemoryInput	true	"Clear options"
//	@Success		200			{object}	router.Response{data=memuc.ClearMemoryResult}	"Memory cleared successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/api/v0/memory/{memory_ref}/{key}/clear [post]
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

	// Parse request body
	var input memuc.ClearMemoryInput
	if err := c.ShouldBindJSON(&input); err != nil {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			"invalid request body",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return
	}

	// Execute use case
	uc := memuc.NewClearMemory(memCtx.Manager, memCtx.MemoryRef, memCtx.Key, &input)
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
//	@Param			key			path		string	true	"Memory key"		example("user:123:profile")
//	@Param			limit		query		int		false	"Limit for role distribution calculation (default: 100, max: 10000)"	example(100)
//	@Param			offset		query		int		false	"Offset for role distribution calculation"	example(0)
//	@Success		200			{object}	router.Response{data=memuc.StatsMemoryOutput}	"Memory statistics retrieved successfully"
//	@Failure		400			{object}	router.Response{error=router.ErrorInfo}	"Invalid request"
//	@Failure		500			{object}	router.Response{error=router.ErrorInfo}	"Internal server error"
//	@Router			/api/v0/memory/{memory_ref}/{key}/stats [get]
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
	uc := &memuc.StatsMemory{
		Manager: memCtx.Manager,
		Worker:  memCtx.Worker,
	}

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
