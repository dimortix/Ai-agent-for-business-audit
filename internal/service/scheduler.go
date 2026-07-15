package service

import (
	"context"
	"errors"
	"time"
)

// RunScheduler — фоновый пересчёт прогнозов и рекомендаций для всех участников
// группы B: первый прогон через 15 секунд после старта, далее по интервалу.
func (s *Service) RunScheduler(ctx context.Context, interval time.Duration, notifier *Notifier) {
	s.log.Info("планировщик пересчёта запущен", "interval", interval)

	initial := time.NewTimer(15 * time.Second)
	defer initial.Stop()
	select {
	case <-ctx.Done():
		return
	case <-initial.C:
		s.recalcAll(ctx, notifier)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.recalcAll(ctx, notifier)
		}
	}
}

func (s *Service) recalcAll(ctx context.Context, notifier *Notifier) {
	participants, err := s.repo.ListParticipantsByGroup(ctx, "B")
	if err != nil {
		s.log.Error("планировщик: не удалось получить участников", "err", err)
		return
	}
	for i := range participants {
		p := participants[i]
		res, err := s.Recalculate(ctx, p.ID)
		if err != nil {
			if errors.Is(err, ErrNoData) {
				continue // у участника ещё нет транзакций — это нормально
			}
			s.log.Error("планировщик: пересчёт не удался", "participant", p.ID, "err", err)
			continue
		}
		if notifier != nil {
			notifier.NotifyIfCritical(ctx, &p, res)
		}
	}
}
