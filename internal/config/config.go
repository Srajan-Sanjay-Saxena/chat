package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"

	constants "chat-v2/internal/constants"
)

var Configuration *Config

type Config struct {
	Env              string   `env:"ENV" envDefault:"development"`
	Port             string   `env:"PORT" envDefault:"8080"`
	DBSource         string   `env:"DB_SOURCE,required"`
	JWTSecret        string   `env:"JWT_SECRET,required" validate:"min=32,max=256,excludesall= \t\n\r"`
	RedisAddr        string   `env:"REDIS_ADDR"`
	RedisUsername    string   `env:"REDIS_USERNAME" envDefault:"default"`
	RedisPassword    string   `env:"REDIS_PASSWORD" validate:"omitempty,min=16,excludesall= \t\n\r"`
	RedisDB          int      `env:"REDIS_DB" envDefault:"0"`
	RedisTLS         bool     `env:"REDIS_TLS" envDefault:"false"`
	WSAllowedOrigins []string `env:"WS_ALLOWED_ORIGINS" envSeparator:","`
}

func init(){
	var err error
	path := constants.EnvPath;
	
	Configuration, err = Load(path)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
}


func Load(envPath string) (*Config, error) {
	if err := godotenv.Load(envPath); err != nil {
		log.Printf("no .env file loaded from %s: %v", envPath, err)
	}

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	validate := validator.New()
	if err := validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
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
