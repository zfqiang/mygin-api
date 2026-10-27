package config

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Config struct {
	Mode        string
	Port        string
	JWTSecret   []byte
	JWTIssuer   string
	JWTTTL      time.Duration
	CORSOrigins []string
	RateLimit   int64
	RateWindow  time.Duration
	MySQL       MySQLConfig
	Redis       RedisConfig
}

type MySQLConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type AppConfig struct {
	Engine      *gin.Engine
	Cfg         *Config
	DB          *gorm.DB
	RedisClient *redis.Client
	Logger      *slog.Logger
}

func Load() (Config, error) {
	var err error
	envPath := LoadEnvFromRoot()
	log.Printf("Loading .env file from %s", envPath)
	err = godotenv.Load(envPath)
	if err != nil {
		log.Fatal("Error loading .env file")
		return Config{}, err
	}

	cfg := Config{
		Mode:        getEnv("APP_MODE", "debug"),
		Port:        getEnv("APP_PORT", "8080"),
		JWTSecret:   []byte(getEnv("JWT_SECRET", "")),
		JWTIssuer:   getEnv("JWT_ISSUER", "mygin-api"),
		JWTTTL:      24 * time.Hour,
		CORSOrigins: splitCSV(getEnv("CORS_ORIGINS", "http://localhost:3000")),
		RateLimit:   100,
		RateWindow:  time.Minute,
		MySQL: MySQLConfig{
			Host:     getEnv("MYSQL_HOST", "127.0.0.1"),
			Port:     getEnv("MYSQL_PORT", "3306"),
			User:     getEnv("MYSQL_USER", "root"),
			Password: getEnv("MYSQL_PASSWORD", ""),
			Database: getEnv("MYSQL_DATABASE", "mygin_api"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       0,
		},
	}

	if cfg.JWTTTL, err = durationEnv("JWT_TTL", cfg.JWTTTL); err != nil {
		return Config{}, err
	}
	if cfg.RateLimit, err = int64Env("RATE_LIMIT", cfg.RateLimit); err != nil {
		return Config{}, err
	}
	if cfg.RateWindow, err = durationEnv("RATE_WINDOW", cfg.RateWindow); err != nil {
		return Config{}, err
	}
	if cfg.Redis.DB, err = intEnv("REDIS_DB", cfg.Redis.DB); err != nil {
		return Config{}, err
	}

	if cfg.Mode != "debug" && cfg.Mode != "release" && cfg.Mode != "test" {
		return Config{}, fmt.Errorf("APP_MODE must be debug, release, or test")
	}
	port, err := strconv.Atoi(cfg.Port)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("APP_PORT must be an integer between 1 and 65535")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must be at least 32 bytes")
	}
	if cfg.JWTTTL <= 0 {
		return Config{}, fmt.Errorf("JWT_TTL must be greater than zero")
	}
	if cfg.RateLimit <= 0 {
		return Config{}, fmt.Errorf("RATE_LIMIT must be greater than zero")
	}
	if cfg.RateWindow <= 0 {
		return Config{}, fmt.Errorf("RATE_WINDOW must be greater than zero")
	}
	if cfg.RateWindow.Milliseconds() < 1 {
		return Config{}, fmt.Errorf("RATE_WINDOW must be at least 1ms")
	}
	if cfg.Redis.DB < 0 {
		return Config{}, fmt.Errorf("REDIS_DB must not be negative")
	}
	if cfg.MySQL.Host == "" || cfg.MySQL.Port == "" || cfg.MySQL.User == "" || cfg.MySQL.Database == "" {
		return Config{}, fmt.Errorf("MYSQL_HOST, MYSQL_PORT, MYSQL_USER, and MYSQL_DATABASE must not be empty")
	}
	mysqlPort, err := strconv.Atoi(cfg.MySQL.Port)
	if err != nil || mysqlPort < 1 || mysqlPort > 65535 {
		return Config{}, fmt.Errorf("MYSQL_PORT must be an integer between 1 and 65535")
	}
	if cfg.Redis.Addr == "" {
		return Config{}, fmt.Errorf("REDIS_ADDR must not be empty")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	items := strings.Split(value, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}

func int64Env(key string, fallback int64) (int64, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}

func intEnv(key string, fallback int) (int, error) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}
