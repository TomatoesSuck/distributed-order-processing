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

	"github.com/TomatoesSuck/distributed-order-processing/order-service/internal/config"
	"github.com/TomatoesSuck/distributed-order-processing/order-service/internal/db"
	"github.com/TomatoesSuck/distributed-order-processing/order-service/internal/handler"
	"github.com/TomatoesSuck/distributed-order-processing/order-service/internal/model"
	"github.com/TomatoesSuck/distributed-order-processing/order-service/internal/repository"
	"github.com/TomatoesSuck/distributed-order-processing/order-service/internal/service"
)

func main() {
	cfg := config.Load()

	database, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("service=order db connect: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		log.Fatalf("service=order get sql.DB: %v", err)
	}
	defer sqlDB.Close()

	if err := database.AutoMigrate(&model.Order{}); err != nil {
		log.Fatalf("service=order automigrate: %v", err)
	}

	repo := repository.NewOrderRepository(database)
	svc := service.NewOrderService(repo)
	h := handler.NewOrderHandler(svc)

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "order"})
	})
	h.Register(r)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Printf("service=order port=%s starting", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("service=order listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("service=order forced shutdown: %v", err)
	}
	log.Println("service=order exited")
}
