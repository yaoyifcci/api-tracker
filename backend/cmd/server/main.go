package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/yaoyi/aitrace/internal/api"
	"github.com/yaoyi/aitrace/internal/config"
	"github.com/yaoyi/aitrace/internal/proxy"
	"github.com/yaoyi/aitrace/internal/storage"
)

func main() {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	store, err := storage.NewStore(cfg.MongoURI, cfg.MongoDB)
	if err != nil {
		log.Fatalf("connect mongodb: %v", err)
	}
	log.Println("connected to MongoDB")

	proxyHandler := proxy.NewHandler(cfg, store)
	apiHandler := api.NewAPI(store)

	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"*"},
		AllowCredentials: false,
	}))

	// REST API for frontend
	apiGroup := r.Group("/api")
	{
		apiGroup.GET("/requests", apiHandler.ListRequests)
		apiGroup.GET("/requests/:id", apiHandler.GetRequest)
		apiGroup.GET("/stats", apiHandler.GetStats)
	}

	// proxy routes — /v1/* uses default endpoint
	r.Any("/v1/*path", proxyHandler.Handle)

	// proxy routes — /:name/* uses named endpoint
	// Register known endpoint names to avoid catching /api/* and /v1/*
	for _, ep := range cfg.Endpoints {
		name := ep.Name
		r.Any("/"+name+"/*path", func(c *gin.Context) {
			c.Set("endpoint_name", name)
			proxyHandler.Handle(c)
		})
	}

	addr := fmt.Sprintf(":%d", cfg.ProxyPort)
	log.Printf("aitrace proxy listening on %s", addr)
	if err := r.Run(addr); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
