// Package bot — Telegram-бот (long polling): привязка номера, /status, /advice,
// доставка OTP-кодов и тревожных уведомлений.
package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/shopspring/decimal"

	"alfa-pulse/internal/models"
	"alfa-pulse/internal/repository"
	"alfa-pulse/internal/service"
)

type Bot struct {
	api  *tgbotapi.BotAPI
	repo *repository.Repository
	log  *slog.Logger
}

// New возвращает (nil, nil), если токен пуст — бот просто выключен.
func New(token string, repo *repository.Repository, log *slog.Logger) (*Bot, error) {
	if token == "" {
		return nil, nil
	}
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("подключение к Telegram: %w", err)
	}
	log.Info("telegram-бот подключен", "username", api.Self.UserName)
	return &Bot{api: api, repo: repo, log: log}, nil
}

// Username — имя бота для ссылки t.me/<username>.
func (b *Bot) Username() string {
	if b == nil {
		return ""
	}
	return b.api.Self.UserName
}

// SendTo реализует service.TelegramSender.
func (b *Bot) SendTo(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = tgbotapi.ModeHTML
	_, err := b.api.Send(msg)
	return err
}

// Run — цикл long polling до отмены контекста.
func (b *Bot) Run(ctx context.Context) {
	cfg := tgbotapi.NewUpdate(0)
	cfg.Timeout = 30
	updates := b.api.GetUpdatesChan(cfg)

	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			return
		case upd, ok := <-updates:
			if !ok {
				return
			}
			if upd.Message == nil {
				continue
			}
			b.handleMessage(ctx, upd.Message)
		}
	}
}

func (b *Bot) handleMessage(ctx context.Context, m *tgbotapi.Message) {
	switch {
	case m.Contact != nil:
		b.handleContact(ctx, m)
	case m.IsCommand():
		b.handleCommand(ctx, m)
	default:
		b.reply(m.Chat.ID, "Не понял 🤔 Наберите /help — покажу, что умею.")
	}
}

func (b *Bot) handleCommand(ctx context.Context, m *tgbotapi.Message) {
	switch m.Command() {
	case "start":
		b.handleStart(m)
	case "status":
		b.handleStatus(ctx, m)
	case "advice":
		b.handleAdvice(ctx, m)
	case "help":
		b.reply(m.Chat.ID,
			"<b>Альфа-Пульс</b> — пульс вашего бизнеса.\n\n"+
				"/status — индекс здоровья, выручка и прогноз\n"+
				"/advice — актуальные советы\n"+
				"/start — привязать номер телефона\n\n"+
				"Также сюда приходят коды входа и тревожные уведомления.")
	default:
		b.reply(m.Chat.ID, "Не знаю такой команды. /help — список команд.")
	}
}

func (b *Bot) handleStart(m *tgbotapi.Message) {
	msg := tgbotapi.NewMessage(m.Chat.ID,
		"Привет! Я «Альфа-Пульс» — слежу за здоровьем вашего бизнеса.\n\n"+
			"Нажмите кнопку ниже, чтобы привязать номер телефона участника пилота.")
	btn := tgbotapi.NewKeyboardButtonContact("📱 Поделиться номером")
	kb := tgbotapi.NewReplyKeyboard([]tgbotapi.KeyboardButton{btn})
	kb.OneTimeKeyboard = true
	kb.ResizeKeyboard = true
	msg.ReplyMarkup = kb
	if _, err := b.api.Send(msg); err != nil {
		b.log.Warn("telegram: не удалось отправить /start", "err", err)
	}
}

func (b *Bot) handleContact(ctx context.Context, m *tgbotapi.Message) {
	phone, err := service.NormalizePhone(m.Contact.PhoneNumber)
	if err != nil {
		b.reply(m.Chat.ID, "Не удалось разобрать номер телефона.")
		return
	}
	p, err := b.repo.GetParticipantByPhone(ctx, phone)
	if errors.Is(err, repository.ErrNotFound) {
		b.reply(m.Chat.ID, "Номер "+phone+" не найден среди участников пилота. Обратитесь к организаторам.")
		return
	}
	if err != nil {
		b.log.Warn("telegram: ошибка поиска участника", "err", err)
		return
	}
	if err := b.repo.SetTelegramChatID(ctx, p.ID, m.Chat.ID); err != nil {
		b.log.Warn("telegram: не удалось сохранить chat_id", "err", err)
		return
	}
	name := p.Name
	if name == "" {
		name = "ваш бизнес"
	}
	b.reply(m.Chat.ID, "Готово! ✅ Номер привязан к «"+name+"».\n\nТеперь сюда будут приходить коды входа и уведомления. Попробуйте /status.")
}

func (b *Bot) handleStatus(ctx context.Context, m *tgbotapi.Message) {
	p := b.participantOrHint(ctx, m.Chat.ID)
	if p == nil {
		return
	}

	var sb strings.Builder
	name := p.Name
	if name == "" {
		name = "Ваш бизнес"
	}
	sb.WriteString("📊 <b>" + name + "</b>\n\n")

	metrics, err := b.repo.GetLastMetrics(ctx, p.ID, 1)
	if err == nil && len(metrics) == 1 {
		mday := metrics[0]
		sb.WriteString(fmt.Sprintf("Выручка за %s: <b>%s</b> (%d покупок)\n",
			service.FormatDateRU(mday.Date), service.FormatMoney(mday.Net()), mday.TransactionCount))
	} else {
		sb.WriteString("Данных о выручке пока нет.\n")
	}

	if p.GroupType != "B" {
		sb.WriteString("\nВы в контрольной группе пилота: прогнозы и советы недоступны.")
		b.reply(m.Chat.ID, sb.String())
		return
	}

	preds, err := b.repo.GetLatestPredictionBatch(ctx, p.ID)
	if err != nil || len(preds) == 0 {
		sb.WriteString("\nПрогноз ещё не рассчитан — загляните позже.")
		b.reply(m.Chat.ID, sb.String())
		return
	}

	if idx := preds[0].HealthIndex; idx != nil {
		sb.WriteString(fmt.Sprintf("Индекс здоровья: <b>%d</b>/100 %s\n", *idx, healthEmoji(*idx)))
	}
	week := decimal.Zero
	for i, pr := range preds {
		if i >= 7 {
			break
		}
		week = week.Add(pr.PredictedRevenue)
	}
	sb.WriteString("Прогноз выручки на 7 дней: ~<b>" + service.FormatMoney(week) + "</b>\n")

	if gap := preds[0].CashGapDate; gap != nil {
		sb.WriteString("⚠️ Возможен кассовый разрыв <b>" + service.FormatDateRU(*gap) + "</b> — загляните в советы: /advice")
	} else {
		sb.WriteString("Кассовых разрывов в ближайшие 14 дней не видно ✅")
	}
	b.reply(m.Chat.ID, sb.String())
}

func (b *Bot) handleAdvice(ctx context.Context, m *tgbotapi.Message) {
	p := b.participantOrHint(ctx, m.Chat.ID)
	if p == nil {
		return
	}
	if p.GroupType != "B" {
		b.reply(m.Chat.ID, "Вы в контрольной группе пилота: советы недоступны.")
		return
	}
	recs, err := b.repo.ListRecommendations(ctx, p.ID, "active")
	if err != nil {
		b.log.Warn("telegram: не удалось получить советы", "err", err)
		return
	}
	if len(recs) == 0 {
		b.reply(m.Chat.ID, "Актуальных советов нет — бизнес в порядке ✅")
		return
	}
	var sb strings.Builder
	sb.WriteString("💡 <b>Что сделать в первую очередь:</b>\n")
	for i, r := range recs {
		if i >= 3 {
			break
		}
		sb.WriteString(fmt.Sprintf("\n%d. %s\n", i+1, r.Message))
	}
	sb.WriteString("\nОтметить выполненным можно в приложении.")
	b.reply(m.Chat.ID, sb.String())
}

func (b *Bot) participantOrHint(ctx context.Context, chatID int64) *models.Participant {
	p, err := b.repo.GetParticipantByChatID(ctx, chatID)
	if errors.Is(err, repository.ErrNotFound) {
		b.reply(chatID, "Сначала привяжите номер телефона: /start")
		return nil
	}
	if err != nil {
		b.log.Warn("telegram: ошибка поиска по chat_id", "err", err)
		return nil
	}
	return p
}

func (b *Bot) reply(chatID int64, text string) {
	if err := b.SendTo(chatID, text); err != nil {
		b.log.Warn("telegram: не удалось отправить сообщение", "err", err)
	}
}

func healthEmoji(idx int) string {
	switch {
	case idx < 40:
		return "🔴"
	case idx < 70:
		return "🟡"
	default:
		return "🟢"
	}
}
