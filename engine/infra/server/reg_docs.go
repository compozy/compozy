package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/compozy/compozy/docs"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const swaggerModelsExpandDepthCollapsed = -1

// setupSwaggerAndDocs wires up Swagger UI and the OpenAPI endpoint with the correct runtime prefix.
func setupSwaggerAndDocs(router *gin.Engine, prefixURL string) {
	configureSwaggerInfo(prefixURL)
	registerDocsUI(router)
	registerSwaggerRedirect(router)
	registerOpenAPIJSON(router)
}

// configureSwaggerInfo synchronizes the generated swagger metadata with the runtime prefix.
func configureSwaggerInfo(prefixURL string) {
	docs.SwaggerInfo.BasePath = prefixURL
	docs.SwaggerInfo.Host = ""
	docs.SwaggerInfo.Schemes = []string{"http", "https"}
}

// registerDocsUI attaches the Swagger UI route pointing to the OpenAPI document.
func registerDocsUI(router *gin.Engine) {
	router.GET("/docs/*any", ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		ginSwagger.URL("/openapi.json"),
		ginSwagger.InstanceName(docs.SwaggerInfo.InstanceName()),
		ginSwagger.DefaultModelsExpandDepth(swaggerModelsExpandDepthCollapsed),
	))
}

// registerSwaggerRedirect keeps backward compatibility for the legacy swagger path.
func registerSwaggerRedirect(router *gin.Engine) {
	router.GET("/swagger/index.html", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/docs/index.html")
	})
}

// registerOpenAPIJSON exposes the OpenAPI 3.0 document converted from the swagger specification.
func registerOpenAPIJSON(router *gin.Engine) {
	router.GET("/openapi.json", openAPIHandler())
}

// openAPIHandler converts the swagger v2 document into OpenAPI v3 on the fly.
func openAPIHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		raw, errResp := loadSwaggerDocument(ctx)
		if errResp != nil {
			respondWithError(c, errResp)
			return
		}
		payload, errResp := convertSwaggerToOpenAPI(ctx, raw, c.Request.Host)
		if errResp != nil {
			respondWithError(c, errResp)
			return
		}
		c.Data(http.StatusOK, "application/json; charset=utf-8", payload)
	}
}

func loadSwaggerDocument(ctx context.Context) ([]byte, *handlerError) {
	log := logger.FromContext(ctx)
	raw := docs.SwaggerInfo.ReadDoc()
	if raw != "" && json.Valid([]byte(raw)) {
		return []byte(raw), nil
	}
	fileBytes, err := os.ReadFile("docs/swagger.json")
	if err != nil {
		log.Error("swagger v2 JSON not available or invalid", "error", err)
		return nil, &handlerError{
			status:  http.StatusInternalServerError,
			message: "swagger spec not available",
			details: "swagger v2 JSON not available or invalid",
		}
	}
	if !json.Valid(fileBytes) {
		log.Error("swagger v2 JSON not available or invalid")
		return nil, &handlerError{
			status:  http.StatusInternalServerError,
			message: "swagger spec not available",
			details: "swagger v2 JSON not available or invalid",
		}
	}
	return fileBytes, nil
}

func convertSwaggerToOpenAPI(ctx context.Context, raw []byte, host string) ([]byte, *handlerError) {
	log := logger.FromContext(ctx)
	var v2 openapi2.T
	if err := json.Unmarshal(raw, &v2); err != nil {
		log.Error("failed to unmarshal swagger v2 JSON", "error", err)
		return nil, &handlerError{
			status:  http.StatusInternalServerError,
			message: "failed to unmarshal swagger v2",
			details: err.Error(),
		}
	}
	if v2.Host == "" {
		v2.Host = host
	}
	v3, err := openapi2conv.ToV3(&v2)
	if err != nil {
		log.Error("failed to convert swagger v2 to openapi v3", "error", err)
		return nil, &handlerError{
			status:  http.StatusInternalServerError,
			message: "failed to convert to openapi v3",
			details: err.Error(),
		}
	}
	data, err := json.MarshalIndent(v3, "", "  ")
	if err != nil {
		log.Error("failed to marshal openapi v3", "error", err)
		return nil, &handlerError{
			status:  http.StatusInternalServerError,
			message: "failed to marshal openapi v3",
			details: err.Error(),
		}
	}
	return data, nil
}

func respondWithError(c *gin.Context, err *handlerError) {
	c.JSON(err.status, gin.H{
		"error":   err.message,
		"details": err.details,
	})
}

type handlerError struct {
	status  int
	message string
	details string
}
