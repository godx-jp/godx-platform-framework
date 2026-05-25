package scheduler

import (
	"os"
	"sync"
	"time"
)

var (
	maintenanceMu sync.RWMutex
	maintenanceOn bool
)

// MaintenanceMode skips all scheduled runs when enabled.
func MaintenanceMode() bool {
	maintenanceMu.RLock()
	defer maintenanceMu.RUnlock()
	return maintenanceOn
}

// SetMaintenanceMode toggles global maintenance (Laravel-style down).
func SetMaintenanceMode(on bool) {
	maintenanceMu.Lock()
	maintenanceOn = on
	maintenanceMu.Unlock()
}

// CurrentEnvironment returns APP_ENV or "production".
func CurrentEnvironment() string {
	if v := os.Getenv("APP_ENV"); v != "" {
		return v
	}
	return "production"
}

func envAllowed(allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	cur := CurrentEnvironment()
	for _, e := range allowed {
		if e == cur {
			return true
		}
	}
	return false
}

func withinBetween(now time.Time, start, end string) bool {
	if start == "" && end == "" {
		return true
	}
	layout := "15:04"
	var startT, endT time.Time
	var err error
	if start != "" {
		startT, err = time.Parse(layout, start)
		if err != nil {
			return true
		}
	}
	if end != "" {
		endT, err = time.Parse(layout, end)
		if err != nil {
			return true
		}
	}
	nowT, _ := time.Parse(layout, now.Format(layout))
	if start != "" && end != "" {
		if startT.Before(endT) || startT.Equal(endT) {
			return !nowT.Before(startT) && !nowT.After(endT)
		}
		return !nowT.Before(startT) || !nowT.After(endT)
	}
	if start != "" {
		return !nowT.Before(startT)
	}
	return !nowT.After(endT)
}
