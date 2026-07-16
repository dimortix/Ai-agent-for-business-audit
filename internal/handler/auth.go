package handler

import (
	"errors"
	"net/http"

	"alfa-pulse/internal/auth"
	"alfa-pulse/internal/repository"
	"alfa-pulse/internal/service"
)

// POST /api/auth/request-code {"phone":"+79..."}
func (d Deps) requestCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	phone, err := service.NormalizePhone(req.Phone)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный номер телефона")
		return
	}

	p, err := d.Repo.GetParticipantByPhone(r.Context(), phone)
	if errors.Is(err, repository.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "Номер не найден среди участников пилота. Обратитесь к организаторам.")
		return
	}
	if err != nil {
		d.Log.Error("поиск участника", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}

	code, err := d.OTP.Generate(r.Context(), phone)
	if errors.Is(err, auth.ErrRateLimited) {
		writeErr(w, http.StatusTooManyRequests, err.Error())
		return
	}
	if err != nil {
		d.Log.Error("генерация OTP", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}

	sentToTelegram := false
	if p.TelegramChatID != nil && d.TG != nil {
		if err := d.TG.SendTo(*p.TelegramChatID, "🔐 Код входа в Альфа.Пульс: <b>"+code+"</b>\nДействует 5 минут."); err != nil {
			d.Log.Warn("не удалось отправить OTP в Telegram", "err", err)
		} else {
			sentToTelegram = true
		}
	}

	resp := map[string]any{"message": "Код отправлен в Telegram"}
	if !sentToTelegram {
		resp["message"] = "Код сгенерирован"
	}
	if d.Cfg.IsDev() {
		// Только для dev/демо: без настроенного бота код иначе не получить.
		resp["debug_code"] = code
		d.Log.Info("OTP выдан (dev)", "phone", phone, "code", code)
	} else {
		d.Log.Info("OTP выдан", "phone", phone, "telegram", sentToTelegram)
	}
	writeJSON(w, http.StatusOK, resp)
}

// POST /api/auth/verify {"phone":"...","code":"1234"}
func (d Deps) verifyCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	phone, err := service.NormalizePhone(req.Phone)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "некорректный номер телефона")
		return
	}

	switch err := d.OTP.Verify(r.Context(), phone, req.Code); {
	case errors.Is(err, auth.ErrTooManyAttempts):
		writeErr(w, http.StatusTooManyRequests, err.Error())
		return
	case errors.Is(err, auth.ErrCodeInvalid):
		writeErr(w, http.StatusUnauthorized, err.Error())
		return
	case err != nil:
		d.Log.Error("проверка OTP", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}

	p, err := d.Repo.GetParticipantByPhone(r.Context(), phone)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}

	access, refresh, err := d.Tokens.IssuePair(r.Context(), p.ID, p.GroupType)
	if err != nil {
		d.Log.Error("выпуск токенов", "err", err)
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	d.setAuthCookies(w, access, refresh)

	writeJSON(w, http.StatusOK, map[string]any{
		"participant": map[string]string{
			"name":       p.Name,
			"phone":      p.Phone,
			"group_type": p.GroupType,
		},
	})
}

// POST /api/auth/refresh — ротация пары токенов по refresh-cookie.
func (d Deps) refreshTokens(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(auth.RefreshCookie)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "нет refresh-токена")
		return
	}
	pid, group, err := d.Tokens.RotateRefresh(r.Context(), cookie.Value)
	if err != nil {
		d.clearAuthCookies(w)
		writeErr(w, http.StatusUnauthorized, "сессия истекла, войдите заново")
		return
	}
	access, refresh, err := d.Tokens.IssuePair(r.Context(), pid, group)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "внутренняя ошибка")
		return
	}
	d.setAuthCookies(w, access, refresh)
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/auth/logout
func (d Deps) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(auth.RefreshCookie); err == nil {
		d.Tokens.RevokeRefresh(r.Context(), cookie.Value)
	}
	d.clearAuthCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (d Deps) setAuthCookies(w http.ResponseWriter, access, refresh string) {
	secure := !d.Cfg.IsDev() // на localhost (http) Secure-cookie не работали бы
	http.SetCookie(w, &http.Cookie{
		Name: auth.AccessCookie, Value: access, Path: "/",
		MaxAge: int(auth.AccessTTL.Seconds()), HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name: auth.RefreshCookie, Value: refresh, Path: "/api/auth",
		MaxAge: int(auth.RefreshTTL.Seconds()), HttpOnly: true, Secure: secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (d Deps) clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: auth.AccessCookie, Value: "", Path: "/", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: auth.RefreshCookie, Value: "", Path: "/api/auth", MaxAge: -1})
}
