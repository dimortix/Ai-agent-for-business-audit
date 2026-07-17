package handler

import (
	"errors"
	"net/http"
	"time"

	"alfa-pulse/internal/auth"
	"alfa-pulse/internal/service"
)

const (
	chatHourlyLimit = 40 // сообщений в час на участника (LLM — дорогой ресурс)
)

// POST /api/chat {"messages":[{"role":"user","content":"..."}, ...]} → {"reply":"..."}
// Чат-советник: LLM с контекстом бизнеса участника.
func (d Deps) chat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pid := auth.ParticipantID(ctx)

	var req struct {
		Messages []service.ChatTurn `json:"messages"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if len(req.Messages) == 0 {
		writeErr(w, http.StatusBadRequest, "нужно хотя бы одно сообщение")
		return
	}

	// Лимит на участника: LLM-запросы дорогие.
	if d.RDB != nil {
		key := "chat_rl:" + pid.String()
		n, err := d.RDB.Incr(ctx, key).Result()
		if err == nil {
			if n == 1 {
				d.RDB.Expire(ctx, key, time.Hour)
			}
			if n > chatHourlyLimit {
				writeErr(w, http.StatusTooManyRequests,
					"слишком много сообщений советнику — продолжите через час")
				return
			}
		}
	}

	reply, err := d.Svc.ChatReply(ctx, pid, req.Messages)
	switch {
	case errors.Is(err, service.ErrLLMDisabled):
		writeErr(w, http.StatusServiceUnavailable, err.Error())
		return
	case err != nil:
		d.Log.Warn("чат-советник: ошибка LLM", "participant", pid, "err", err)
		writeErr(w, http.StatusBadGateway, "советник сейчас недоступен, попробуйте ещё раз")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"reply": reply})
}
