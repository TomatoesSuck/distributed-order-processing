package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	Port               string
	DBHost             string
	DBPort             string
	DBUser             string
	DBPass             string
	DBName             string
	RabbitMQURL        string
	PaymentFailureRate float64
}

func Load() Config {
	return Config{
		Port:               getEnv("PAYMENT_PORT", "8083"),
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             getEnv("DB_PORT", "3306"),
		DBUser:             getEnv("DB_USER", "root"),
		DBPass:             getEnv("DB_PASS", "rootpw"),
		DBName:             getEnv("DB_NAME", "payments_db"),
		RabbitMQURL:        getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		PaymentFailureRate: getEnvFloat("PAYMENT_FAILURE_RATE", 0.0),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		log.Printf("config: invalid %s=%q, using default %v: %v", key, v, fallback, err)
		return fallback
	}
	return f
}
