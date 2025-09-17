package server

import (
	"net/http"

	"github.com/compozy/compozy/docs"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const swaggerModelsExpandDepthCollapsed = -1

func setupSwaggerAndDocs(router *gin.Engine, prefixURL string) {
	docs.SwaggerInfo.BasePath = prefixURL
	docs.SwaggerInfo.Host = ""
	docs.SwaggerInfo.Schemes = []string{"http", "https"}
	url := ginSwagger.URL("/swagger/doc.json")
	router.GET("/swagger-ui", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	router.GET("/docs-ui", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/docs/index.html")
	})
	router.GET("/swagger/*any", ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		url,
		ginSwagger.DefaultModelsExpandDepth(swaggerModelsExpandDepthCollapsed),
	))
	router.GET("/docs/*any", ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		url,
		ginSwagger.DefaultModelsExpandDepth(swaggerModelsExpandDepthCollapsed),
	))
}
