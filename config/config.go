package config

import (
	"os"
	"strconv"
	"strings"
	"fmt"
	"github.com/joho/godotenv"
)

type Config struct {
	Port             string
	DBSource         string
	JWTSecret        string
	RedisAddr        string
	RedisUsername    string
	RedisPassword    string
	RedisDB          int
	WSAllowedOrigins []string
}

func LoadConfig(envPath string) (*Config, error) {

	if envPath == "" {
		envPath = ".env"
	}
	godotenv.Load(envPath) // Load .env file, ignore error if it doesn't exist
	
	redisDBInt, err := strconv.Atoi(getEnv("REDIS_DB", "0"))
	if err != nil {
		redisDBInt = 0
	}


	cfg := &Config{
		Port:             getEnv("PORT", "8080"),
		DBSource:         getEnv("DB_SOURCE", ""),
		JWTSecret:        getEnv("JWT_SECRET", ""),
		RedisAddr:        getEnv("REDIS_ADDR", ""),
		RedisUsername:    getEnv("REDIS_USERNAME", "default"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		RedisDB:          redisDBInt,
		WSAllowedOrigins: ParseAllowedOrigins(getEnv("WS_ALLOWED_ORIGINS", "")),
	}

	if cfg.DBSource == "" {
		return nil, fmt.Errorf("DB_SOURCE is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.RedisAddr == "" {
		return nil, fmt.Errorf("REDIS_ADDR is required")
	}
	if cfg.RedisPassword == "" {
		return nil, fmt.Errorf("REDIS_PASSWORD is required")
	}
	
	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func ParseAllowedOrigins(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
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
