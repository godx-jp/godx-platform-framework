package scheduler

import "time"

// JobHealth is the last-known runtime status for one scheduled job.
type JobHealth struct {
	Name       string
	LastRun    time.Time
	LastStatus string
	LastDetail string
}

// LastRun returns the most recent run timestamp for name, if any.
func (s *Scheduler) LastRun(name string) (time.Time, bool) {
	s.healthMu.RLock()
	defer s.healthMu.RUnlock()
	h, ok := s.health[name]
	if !ok || h.LastRun.IsZero() {
		return time.Time{}, false
	}
	return h.LastRun, true
}

// Health returns a snapshot of per-job health records.
func (s *Scheduler) Health() map[string]JobHealth {
	s.healthMu.RLock()
	defer s.healthMu.RUnlock()
	out := make(map[string]JobHealth, len(s.health))
	for k, v := range s.health {
		out[k] = v
	}
	return out
}

func (s *Scheduler) recordHealth(name, status, detail string) {
	s.healthMu.Lock()
	defer s.healthMu.Unlock()
	if s.health == nil {
		s.health = map[string]JobHealth{}
	}
	s.health[name] = JobHealth{
		Name:       name,
		LastRun:    time.Now(),
		LastStatus: status,
		LastDetail: detail,
	}
}
