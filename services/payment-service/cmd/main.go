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

	"github.com/TomatoesSuck/distributed-order-processing/payment-service/internal/config"
	"github.com/TomatoesSuck/distributed-order-processing/payment-service/internal/db"
	"github.com/TomatoesSuck/distributed-order-processing/payment-service/internal/handler"
	"github.com/TomatoesSuck/distributed-order-processing/payment-service/internal/model"
	"github.com/TomatoesSuck/distributed-order-processing/payment-service/internal/repository"
	"github.com/TomatoesSuck/distributed-order-processing/payment-service/internal/service"
)

func main() {
	cfg := config.Load()

	database, err := db.Connect(cfg)
	if err != nil {
		log.Fatalf("service=payment db connect: %v", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		log.Fatalf("service=payment get sql.DB: %v", err)
	}
	defer sqlDB.Close()

	if err := database.AutoMigrate(&model.Payment{}); err != nil {
		log.Fatalf("service=payment automigrate: %v", err)
	}

	repo := repository.NewPaymentRepository(database)
	svc := service.NewPaymentService(repo)
	h := handler.NewPaymentHandler(svc)

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "payment"})
	})
	h.Register(r)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Printf("service=payment port=%s starting", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("service=payment listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Fatalf("service=payment forced shutdown: %v", err)
	}
	log.Println("service=payment exited")
}
