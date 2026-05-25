package observability_test

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/godx-jp/godx-platform-framework/observability"
)

func TestLoadConfigFromEnv_Defaults(t *testing.T) {
	t.Setenv("OBS_BACKEND", "")
	t.Setenv("OBS_LOG_LEVEL", "")
	t.Setenv("OBS_TRACE_SAMPLE", "")
	t.Setenv("DEPLOYMENT_ENVIRONMENT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "")
	t.Setenv("AWS_REGION", "")
	t.Setenv("OBS_LOG_GROUP", "")
	t.Setenv("OBS_LOG_FILE", "")
	t.Setenv("OBS_LOG_ROTATE", "")
	t.Setenv("OBS_LOG_MAX_SIZE_MB", "")
	t.Setenv("OBS_LOG_MAX_AGE_DAYS", "")
	t.Setenv("OBS_LOG_MAX_BACKUPS", "")
	t.Setenv("OBS_LOG_COMPRESS", "")

	cfg := observability.LoadConfigFromEnv()
	if cfg.Backend != observability.BackendStdout {
		t.Errorf("Backend = %q, want %q", cfg.Backend, observability.BackendStdout)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelInfo)
	}
	if cfg.TraceSampleRate != 1.0 {
		t.Errorf("TraceSampleRate = %v, want 1.0", cfg.TraceSampleRate)
	}
	if cfg.Environment != "dev" {
		t.Errorf("Environment = %q, want dev", cfg.Environment)
	}
	if cfg.OTLPProtocol != "grpc" {
		t.Errorf("OTLPProtocol = %q, want grpc", cfg.OTLPProtocol)
	}
	if !cfg.OTLPInsecure {
		t.Errorf("OTLPInsecure = false, want true (default)")
	}
	if cfg.FileRotate != "daily" || cfg.FileMaxSizeMB != 100 || cfg.FileMaxAgeDays != 14 || !cfg.FileCompress {
		t.Errorf("file defaults wrong: %+v", cfg)
	}
}

func TestLoadConfigFromEnv_OverrideAll(t *testing.T) {
	t.Setenv("OBS_BACKEND", "otlp")
	t.Setenv("OBS_LOG_LEVEL", "warn")
	t.Setenv("OBS_TRACE_SAMPLE", "0.25")
	t.Setenv("DEPLOYMENT_ENVIRONMENT", "prod")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "false")
	t.Setenv("AWS_REGION", "ap-northeast-1")
	t.Setenv("OBS_LOG_GROUP", "/svc/my-app")

	cfg := observability.LoadConfigFromEnv()
	if cfg.Backend != "otlp" || cfg.LogLevel != slog.LevelWarn || cfg.TraceSampleRate != 0.25 ||
		cfg.Environment != "prod" || cfg.OTLPEndpoint != "otel:4317" ||
		cfg.OTLPProtocol != "http" || cfg.OTLPInsecure || cfg.AWSRegion != "ap-northeast-1" ||
		cfg.LogGroupName != "/svc/my-app" {
		t.Fatalf("override mismatch: %+v", cfg)
	}
}

func TestConfig_Validate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     observability.Config
		wantErr bool
	}{
		{
			name: "ok stdout",
			cfg: observability.Config{
				ServiceName: "svc",
				Backend:     observability.BackendStdout,
			},
		},
		{
			name: "missing service name",
			cfg: observability.Config{
				Backend: observability.BackendStdout,
			},
			wantErr: true,
		},
		{
			name: "unknown backend",
			cfg: observability.Config{
				ServiceName: "svc",
				Backend:     "voodoo",
			},
			wantErr: true,
		},
		{
			name: "otlp without endpoint",
			cfg: observability.Config{
				ServiceName: "svc",
				Backend:     observability.BackendOTLP,
			},
			wantErr: true,
		},
		{
			name: "file without path",
			cfg: observability.Config{
				ServiceName: "svc",
				Backend:     observability.BackendFile,
			},
			wantErr: true,
		},
		{
			name: "file with path ok",
			cfg: observability.Config{
				ServiceName: "svc",
				Backend:     observability.BackendFile,
				FilePath:    "/tmp/log.log",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate err = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && errors.Is(err, nil) {
				t.Fatalf("nil-check sanity")
			}
		})
	}
}
