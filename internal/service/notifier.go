package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"alfa-pulse/internal/models"
	"alfa-pulse/internal/push"
	"alfa-pulse/internal/repository"
)

// TelegramSender — отправка сообщений в Telegram (реализует internal/bot.Bot).
// Интерфейс, чтобы сервис не зависел от пакета бота (и бот мог отсутствовать).
type TelegramSender interface {
	SendTo(chatID int64, text string) error
}

const criticalIndex = 40

// Notifier шлёт тревожные уведомления при падении ИЖБ в красную зону,
// не чаще раза в сутки на участника (ключ в Redis).
type Notifier struct {
	repo *repository.Repository
	push *push.Sender
	tg   TelegramSender // может быть nil (бот выключен)
	rdb  *redis.Client
	log  *slog.Logger
}

func NewNotifier(repo *repository.Repository, sender *push.Sender, tg TelegramSender,
	rdb *redis.Client, log *slog.Logger) *Notifier {
	return &Notifier{repo: repo, push: sender, tg: tg, rdb: rdb, log: log}
}

// NotifyIfCritical: index < 40 → web-push по всем подпискам + Telegram.
func (n *Notifier) NotifyIfCritical(ctx context.Context, p *models.Participant, res *RecalcResult) {
	if res == nil || res.HealthIndex >= criticalIndex {
		return
	}

	// Антиспам: не чаще одного тревожного уведомления в 24 часа.
	ok, err := n.rdb.SetNX(ctx, "notify:last:"+p.ID.String(), time.Now().Format(time.RFC3339), 24*time.Hour).Result()
	if err != nil {
		n.log.Warn("redis недоступен, уведомление пропущено", "err", err)
		return
	}
	if !ok {
		n.log.Debug("уведомление уже отправлялось за последние сутки", "participant", p.ID)
		return
	}

	title := "Альфа-Пульс: бизнесу нужно внимание"
	body := fmt.Sprintf("Индекс здоровья упал до %d из 100.", res.HealthIndex)
	if res.CashGapDate != nil {
		body += fmt.Sprintf(" Возможен кассовый разрыв %s — не хватит около %s.",
			FormatDateRU(*res.CashGapDate), FormatMoney(res.CashGapAmount))
	}
	body += " Откройте советы в приложении."

	n.sendWebPush(ctx, p, push.Payload{Title: title, Body: body, URL: "/"})
	n.sendTelegram(p, "🚨 <b>"+title+"</b>\n"+body)
}

func (n *Notifier) sendWebPush(ctx context.Context, p *models.Participant, payload push.Payload) {
	if !n.push.Enabled() {
		n.log.Info("web push выключен (нет VAPID-ключей) — уведомление только в лог",
			"participant", p.ID, "body", payload.Body)
		return
	}
	subs, err := n.repo.ListPushSubscriptions(ctx, p.ID)
	if err != nil {
		n.log.Warn("не удалось получить push-подписки", "err", err)
		return
	}
	if len(subs) == 0 {
		n.log.Info("тревога зафиксирована, но push-подписок у участника нет",
			"participant", p.ID, "body", payload.Body)
		return
	}
	for _, sub := range subs {
		err := n.push.Send(ctx, sub, payload)
		switch {
		case errors.Is(err, push.ErrGone):
			_ = n.repo.DeletePushSubscription(ctx, p.ID, sub.Endpoint)
			n.log.Info("мёртвая push-подписка удалена", "participant", p.ID)
		case err != nil:
			n.log.Warn("не удалось отправить web push", "participant", p.ID, "err", err)
		default:
			n.log.Info("web push отправлен", "participant", p.ID)
		}
	}
}

func (n *Notifier) sendTelegram(p *models.Participant, text string) {
	if n.tg == nil || p.TelegramChatID == nil {
		return
	}
	if err := n.tg.SendTo(*p.TelegramChatID, text); err != nil {
		n.log.Warn("не удалось отправить Telegram-уведомление", "participant", p.ID, "err", err)
	} else {
		n.log.Info("telegram-уведомление отправлено", "participant", p.ID)
	}
}
