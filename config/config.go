package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Env              string   `env:"ENV" envDefault:"development"`
	Port             string   `env:"PORT" envDefault:"8080"`
	DBSource         string   `env:"DB_SOURCE,required"`
	JWTSecret        string   `env:"JWT_SECRET,required"`
	RedisAddr        string   `env:"REDIS_ADDR"`
	RedisUsername    string   `env:"REDIS_USERNAME" envDefault:"default"`
	RedisPassword    string   `env:"REDIS_PASSWORD"`
	RedisDB          int      `env:"REDIS_DB" envDefault:"0"`
	WSAllowedOrigins []string `env:"WS_ALLOWED_ORIGINS" envSeparator:","`
}

func LoadConfig(envPath string) (*Config, error) {
	if err := godotenv.Load(envPath); err != nil {
		log.Printf("no .env file loaded from %s: %v", envPath, err)
	}

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}

func ParseAllowedOrigins(raw string) []string {
	if raw == "" {
		return []string{}
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			origins = append(origins, part)
		}
	}
	return origins
}

