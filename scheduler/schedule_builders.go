package scheduler

import (
	"fmt"
	"time"
)

// DailyAt schedules a job daily at HH:MM (24h).
func (s *Scheduler) DailyAt(at string) *Schedule {
	t, err := time.Parse("15:04", at)
	if err != nil {
		return s.Cron("0 0 0 * * *")
	}
	return s.Cron(fmtCronDaily(t.Hour(), t.Minute()))
}

// Hourly schedules a job at the start of every hour.
func (s *Scheduler) Hourly() *Schedule { return s.Cron("0 0 * * * *") }

// WeeklyOn schedules a job weekly on weekday (0=Sunday) at HH:MM.
func (s *Scheduler) WeeklyOn(weekday time.Weekday, at string) *Schedule {
	t, err := time.Parse("15:04", at)
	if err != nil {
		return s.Cron("0 0 0 * * *")
	}
	return s.Cron(fmt.Sprintf("0 %d %d * * %d", t.Minute(), t.Hour(), int(weekday)))
}

func fmtCronDaily(h, m int) string {
	return fmt.Sprintf("0 %d %d * * *", m, h)
}
