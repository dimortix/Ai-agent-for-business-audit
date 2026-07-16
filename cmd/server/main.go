// Альфа.Пульс: основной сервер — HTTP API + SPA, Telegram-бот,
// планировщик пересчёта прогнозов, уведомления.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"alfa-pulse/internal/auth"
	"alfa-pulse/internal/bot"
	"alfa-pulse/internal/config"
	"alfa-pulse/internal/db"
	"alfa-pulse/internal/handler"
	"alfa-pulse/internal/push"
	"alfa-pulse/internal/repository"
	"alfa-pulse/internal/service"
)

func main() {
	cfg := config.Load()

	var logHandler slog.Handler
	if cfg.IsDev() {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	} else {
		logHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	log := slog.New(logHandler)

	if err := run(cfg, log); err != nil {
		log.Error("сервер остановлен с ошибкой", "err", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// PostgreSQL + миграции
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		return err
	}
	log.Info("база данных готова, миграции применены")

	// Redis
	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return err
	}
	rdb := redis.NewClient(redisOpts)
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return err
	}

	repo := repository.New(pool)
	svc := service.New(repo, service.NewMLClient(cfg.MLServiceURL), log)
	// LLM-модуль AI-советов (опционально: GigaChat/OpenAI/локальный эндпоинт)
	if cfg.LLMApiURL != "" {
		svc = svc.WithLLM(service.NewLLMClient(cfg.LLMApiURL, cfg.LLMApiKey, cfg.LLMModel))
		log.Info("AI-советы включены (LLM подключён)", "model", cfg.LLMModel)
	} else {
		log.Info("AI-советы выключены (LLM_API_URL не задан) — работают правила")
	}
	pushSender := push.New(cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, log)
	if !pushSender.Enabled() {
		log.Warn("web push выключен: заполните VAPID_PUBLIC_KEY/VAPID_PRIVATE_KEY (make vapid)")
	}

	// Telegram-бот (опционален)
	var tgSender service.TelegramSender
	tgBot, err := bot.New(cfg.TelegramBotToken, repo, log)
	if err != nil {
		log.Warn("telegram-бот не запустился, продолжаю без него", "err", err)
	}
	if tgBot != nil {
		tgSender = tgBot
		go tgBot.Run(ctx)
	} else {
		log.Info("telegram-бот выключен (TELEGRAM_BOT_TOKEN не задан)")
	}

	notifier := service.NewNotifier(repo, pushSender, tgSender, rdb, log)
	go svc.RunScheduler(ctx, cfg.RecalcInterval, notifier)

	router := handler.NewRouter(handler.Deps{
		Cfg:         cfg,
		Log:         log,
		Repo:        repo,
		Svc:         svc,
		Notifier:    notifier,
		OTP:         auth.NewOTPManager(rdb),
		Tokens:      auth.NewTokenManager(cfg.JWTSecret, rdb),
		Push:        pushSender,
		TG:          tgSender,
		BotUsername: tgBot.Username(),
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("HTTP-сервер запущен", "addr", cfg.HTTPAddr, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Info("получен сигнал остановки, завершаю работу…")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
