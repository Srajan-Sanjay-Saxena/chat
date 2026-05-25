package config

import (
	"log"
	"os"
	"strings"
	"github.com/joho/godotenv"
)

type Config struct {
	Port string
	DbSource string
	JWTSecret string
	RedisAddr string
	WSAllowedOrigins string
}

func LoadConfig() (*Config) {
	err := godotenv.Load()
	if err != nil {
		log.Println(".env file not found")
	}

	return &Config{
		Port: os.Getenv("PORT"),
		DbSource: os.Getenv("dbSource"),
		JWTSecret: os.Getenv("JWT_SECRET"),
		RedisAddr: os.Getenv("REDIS_ADDR"),
		WSAllowedOrigins: os.Getenv("WS_ALLOWED_ORIGINS"),
	}
}

func ParseAllowedOrigins(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			origins = append(origins, part)
		}
	}

	return origins
}