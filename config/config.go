package config

import (
	"log"
	"os"
	"github.com/joho/godotenv"
)

type Config struct {
	Port string
	DbSource string
	JWTSecret string
	RedisAddr string
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
	}
}