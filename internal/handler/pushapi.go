package handler

import (
	"net/http"

	"alfa-pulse/internal/auth"
	"alfa-pulse/internal/models"
)

// GET /api/push/vapid-key — публичный VAPID-ключ для подписки в браузере.
func (d Deps) vapidKey(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": d.Push.Enabled(),
		"key":     d.Push.PublicKey(),
	})
}

// POST /api/push/subscribe — браузерная PushSubscription.
func (d Deps) pushSubscribe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
		Keys     struct {
			P256dh string `json:"p256dh"`
			Auth   string `json:"auth"`
		} `json:"keys"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Endpoint == "" || req.Keys.P256dh == "" || req.Keys.Auth == "" {
		writeErr(w, http.StatusBadRequest, "неполная подписка: нужны endpoint и keys")
		return
	}

	err := d.Repo.UpsertPushSubscription(r.Context(), models.PushSubscription{
		ParticipantID: auth.ParticipantID(r.Context()),
		Endpoint:      req.Endpoint,
		P256dh:        req.Keys.P256dh,
		Auth:          req.Keys.Auth,
	})
	if err != nil {
		d.Log.Error("сохранение push-подписки", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "subscribed"})
}
