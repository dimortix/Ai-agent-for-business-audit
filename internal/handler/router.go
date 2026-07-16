package handler

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"alfa-pulse/internal/auth"
	"alfa-pulse/internal/config"
	"alfa-pulse/internal/push"
	"alfa-pulse/internal/repository"
	"alfa-pulse/internal/service"
)

// Deps — зависимости HTTP-слоя (собираются в cmd/server).
type Deps struct {
	Cfg         config.Config
	Log         *slog.Logger
	Repo        *repository.Repository
	Svc         *service.Service
	Notifier    *service.Notifier
	OTP         *auth.OTPManager
	Tokens      *auth.TokenManager
	Push        *push.Sender
	TG          service.TelegramSender // nil, если бот выключен
	BotUsername string                 // для ссылки привязки t.me/<bot>
}

var httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "alfa_pulse_http_requests_total",
	Help: "HTTP-запросы по методу и классу статуса",
}, []string{"method", "status_class"})

func NewRouter(d Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(requestLogger(d.Log))
	r.Use(chimw.Recoverer)
	r.Use(chimw.Compress(5))
	// Общий rate-limit API по IP (OTP имеет свои, более жёсткие лимиты).
	r.Use(httprate.Limit(240, time.Minute, httprate.WithKeyByIP(),
		httprate.WithLimitHandler(func(w http.ResponseWriter, _ *http.Request) {
			writeErr(w, http.StatusTooManyRequests, "слишком много запросов, попробуйте позже")
		})))
	// Anti-CSRF: SameSite=Strict на cookie + проверка Origin на мутациях.
	r.Use(originCheck)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	// Технические метрики (в проде закрыть на уровне reverse-proxy).
	r.Method(http.MethodGet, "/metrics", promhttp.Handler())

	// Авторизация
	r.Route("/api/auth", func(r chi.Router) {
		r.Post("/request-code", d.requestCode)
		r.Post("/verify", d.verifyCode)
		r.Post("/refresh", d.refreshTokens)
		r.Post("/logout", d.logout)
	})

	// Пользовательское API (JWT в cookie)
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(d.Tokens))
		r.Get("/api/dashboard", d.dashboard)
		r.Get("/api/analytics", d.analytics)
		r.Get("/api/analytics/export", d.exportCSV)
		r.Get("/api/insights", d.insights)
		r.Get("/api/advice", d.listAdvice)
		r.Post("/api/advice/{id}/done", d.adviceDone)
		r.Get("/api/push/vapid-key", d.vapidKey)
		r.Post("/api/push/subscribe", d.pushSubscribe)
		// V2: операции, самостоятельные расходы, месячный отчёт
		r.Get("/api/my/operations", d.operations)
		r.Get("/api/my/expenses", d.myExpenses)
		r.Post("/api/my/expenses/fixed", d.addFixedExpense)
		r.Delete("/api/my/expenses/fixed", d.deleteFixedExpense)
		r.Post("/api/my/expenses/one-off", d.addOneOffExpense)
		r.Delete("/api/my/expenses/one-off/{id}", d.deleteOneOffExpense)
		r.Get("/api/report/monthly", d.monthlyReport)
	})

	// Админское API (X-Admin-Token, действия попадают в аудит-лог)
	r.Group(func(r chi.Router) {
		r.Use(d.adminOnly)
		r.Post("/api/participants/import", d.importParticipants)
		r.Post("/api/admin/import-transactions", d.importTransactions)
		r.Post("/api/admin/recalculate/{participantID}", d.recalculate)
		r.Get("/api/admin/participants", d.adminParticipants)
		r.Get("/api/admin/expenses/{participantID}", d.listExpenses)
		r.Post("/api/admin/expenses/{participantID}", d.upsertExpense)
		r.Delete("/api/admin/expenses/{participantID}", d.deleteExpense)
	})

	// Всё остальное — SPA
	r.NotFound(spaHandler(d.Cfg.WebDir))

	return r
}

// originCheck отклоняет мутирующие запросы с чужим Origin (второй слой
// анти-CSRF поверх SameSite=Strict). Запросы без Origin (curl, same-origin
// навигация) пропускаются.
func originCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if origin := r.Header.Get("Origin"); origin != "" {
				if host := hostOf(origin); host != "" && host != r.Host {
					writeErr(w, http.StatusForbidden, "запрос с чужого источника отклонён")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func hostOf(origin string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(origin, "https://"), "http://")
	return strings.TrimSuffix(s, "/")
}

// requestLogger — компактный лог запросов через slog + счётчик Prometheus.
func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			httpRequests.WithLabelValues(r.Method, statusClass(ww.Status())).Inc()

			// статику и успешные не-API запросы не логируем — шум
			if r.URL.Path == "/healthz" || ww.Status() < 400 && !isAPI(r.URL.Path) {
				return
			}
			log.Info("http",
				"method", r.Method, "path", r.URL.Path,
				"status", ww.Status(), "ms", time.Since(start).Milliseconds())
		})
	}
}

func statusClass(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	default:
		return "2xx"
	}
}

func isAPI(path string) bool {
	return strings.HasPrefix(path, "/api/")
}
