package driver

import "time"

const (
	DriverStdlib    = "stdlib"
	DriverMock      = "mock"
	DriverResilient = "resilient"
)

// Spec configures an httpclient driver.
type Spec struct {
	Name    string
	BaseURL string
	Timeout time.Duration

	// DefaultHeaders applied to every request (driver may merge).
	DefaultHeaders map[string]string

	// Resilient driver options.
	MaxRetries     int
	RetryBackoff   time.Duration
	CBMaxFailures  int
	CBResetTimeout time.Duration

	// OTel service name for span naming (empty → "httpclient").
	OTelServiceName string

	Extra map[string]string
}
