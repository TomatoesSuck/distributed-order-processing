package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/TomatoesSuck/distributed-order-processing/inventory-service/internal/config"
	"github.com/TomatoesSuck/distributed-order-processing/inventory-service/internal/db"
	"github.com/TomatoesSuck/distributed-order-processing/inventory-service/internal/handler"
	"github.com/TomatoesSuck/distributed-order-processing/inventory-service/internal/model"
	"github.com/TomatoesSuck/distributed-order-processing/inventory-service/internal/repository"
	"github.com/TomatoesSuck/distributed-order-processing/inventory-service/internal/service"
)

func main() {
	cfg := config.Load()

	database, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("service=inventory db connect: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		log.Fatalf("service=inventory get sql.DB: %v", err)
	}
	defer sqlDB.Close()

	if err := database.AutoMigrate(&model.Inventory{}); err != nil {
		log.Fatalf("service=inventory automigrate: %v", err)
	}

	repo := repository.NewInventoryRepository(database)

	ctx := context.Background()
	seeds := []struct {
		productID    uint64
		availableQty int
	}{
		{1001, 100},
		{1002, 50},
	}
	for _, s := range seeds {
		if err := repo.SeedIfNotExists(ctx, s.productID, s.availableQty); err != nil {
			log.Fatalf("service=inventory seed product %d: %v", s.productID, err)
		}
	}
	log.Println("service=inventory seed complete")

	svc := service.NewInventoryService(repo)
	h := handler.NewInventoryHandler(svc)

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "inventory"})
	})
	h.Register(r)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Printf("service=inventory port=%s starting", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("service=inventory listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Fatalf("service=inventory forced shutdown: %v", err)
	}
	log.Println("service=inventory exited")
}
