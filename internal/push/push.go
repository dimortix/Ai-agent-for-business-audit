// Package push — отправка Web Push уведомлений (VAPID).
package push

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	webpush "github.com/SherClockHolmes/webpush-go"

	"alfa-pulse/internal/models"
)

// ErrGone: endpoint подписки мёртв (браузер отписался) — подписку надо удалить.
var ErrGone = errors.New("push-подписка недействительна")

// ErrDisabled: VAPID-ключи не заданы.
var ErrDisabled = errors.New("web push выключен: не заданы VAPID-ключи")

type Sender struct {
	pub  string
	priv string
	log  *slog.Logger
}

func New(pub, priv string, log *slog.Logger) *Sender {
	return &Sender{pub: pub, priv: priv, log: log}
}

func (s *Sender) Enabled() bool { return s.pub != "" && s.priv != "" }

func (s *Sender) PublicKey() string { return s.pub }

// Payload обрабатывается сервис-воркером (web/src/sw.ts).
type Payload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
}

func (s *Sender) Send(ctx context.Context, sub models.PushSubscription, p Payload) error {
	if !s.Enabled() {
		return ErrDisabled
	}
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	resp, err := webpush.SendNotificationWithContext(ctx, body, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		VAPIDPublicKey:  s.pub,
		VAPIDPrivateKey: s.priv,
		TTL:             3600,
		Subscriber:      "mailto:alfa-pulse@example.com",
		Urgency:         webpush.UrgencyHigh,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return ErrGone
	case resp.StatusCode >= 400:
		return errors.New("push-сервис вернул статус " + resp.Status)
	}
	return nil
}
