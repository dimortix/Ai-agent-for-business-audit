// Package config загружает конфигурацию приложения из переменных окружения.
package config

import (
	"os"
	"time"
)

type Config struct {
	DatabaseURL      string
	RedisURL         string
	MLServiceURL     string
	HTTPAddr         string
	WebDir           string
	DataDir          string
	JWTSecret        string
	AdminToken       string
	AppEnv           string // dev | prod
	TelegramBotToken string
	VAPIDPublicKey   string
	VAPIDPrivateKey  string
	RecalcInterval   time.Duration

	// LLM для AI-советов (OpenAI-совместимый эндпоинт: GigaChat/OpenAI/локальный).
	// Пусто → советы только по правилам.
	LLMApiURL string
	LLMApiKey string
	LLMModel  string

	// Провайдер эквайринга (боевой источник транзакций). Пусто → только CSV.
	AcquiringAPIURL   string
	AcquiringAPIToken string
}

// Load читает окружение; для всего есть dev-дефолты, чтобы проект
// запускался «из коробки» (docker-compose переопределяет адреса сервисов).
func Load() Config {
	return Config{
		DatabaseURL:      getenv("DATABASE_URL", "postgres://pulse:pulse@localhost:5432/pulse?sslmode=disable"),
		RedisURL:         getenv("REDIS_URL", "redis://localhost:6379/0"),
		MLServiceURL:     getenv("ML_SERVICE_URL", "http://localhost:8000"),
		HTTPAddr:         getenv("HTTP_ADDR", ":8080"),
		WebDir:           getenv("WEB_DIR", "web/dist"),
		DataDir:          getenv("DATA_DIR", "data"),
		JWTSecret:        getenv("JWT_SECRET", "dev-secret-change-me-please"),
		AdminToken:       getenv("ADMIN_TOKEN", "alfa-admin"),
		AppEnv:           getenv("APP_ENV", "dev"),
		TelegramBotToken: getenv("TELEGRAM_BOT_TOKEN", ""),
		VAPIDPublicKey:   getenv("VAPID_PUBLIC_KEY", ""),
		VAPIDPrivateKey:  getenv("VAPID_PRIVATE_KEY", ""),
		RecalcInterval:   getduration("RECALC_INTERVAL", time.Hour),
		LLMApiURL:        getenv("LLM_API_URL", ""),
		LLMApiKey:        getenv("LLM_API_KEY", ""),
		LLMModel:         getenv("LLM_MODEL", ""),
		AcquiringAPIURL:  getenv("ACQUIRING_API_URL", ""),
		AcquiringAPIToken: getenv("ACQUIRING_API_TOKEN", ""),
	}
}

func (c Config) IsDev() bool { return c.AppEnv != "prod" }

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getduration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}
