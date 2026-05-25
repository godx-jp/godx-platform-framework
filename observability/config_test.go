package observability_test

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/godx-jp/godx-platform-framework/observability"
)

func TestLoadConfigFromEnv_Defaults(t *testing.T) {
	t.Setenv("OBSERVABILITY_DRIVER", "")
	t.Setenv("OBSERVABILITY_LOG_LEVEL", "")
	t.Setenv("OBSERVABILITY_TRACE_SAMPLE_RATE", "")
	t.Setenv("DEPLOYMENT_ENVIRONMENT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "")
	t.Setenv("AWS_REGION", "")
	t.Setenv("OBSERVABILITY_CLOUDWATCH_LOG_GROUP", "")
	t.Setenv("OBSERVABILITY_LOG_FILE_PATH", "")
	t.Setenv("OBSERVABILITY_LOG_FILE_ROTATION", "")
	t.Setenv("OBSERVABILITY_LOG_FILE_MAX_SIZE_MB", "")
	t.Setenv("OBSERVABILITY_LOG_FILE_MAX_AGE_DAYS", "")
	t.Setenv("OBSERVABILITY_LOG_FILE_MAX_BACKUPS", "")
	t.Setenv("OBSERVABILITY_LOG_FILE_COMPRESS", "")

	cfg := observability.LoadConfigFromEnv()
	if cfg.Driver != observability.DriverStdout {
		t.Errorf("Driver = %q, want %q", cfg.Driver, observability.DriverStdout)
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
	if cfg.OTLPInsecure {
		t.Errorf("OTLPInsecure = true, want false (secure by default)")
	}
	if cfg.LogFileRotation != "daily" || cfg.LogFileMaxSizeMB != 100 ||
		cfg.LogFileMaxAgeDays != 14 || !cfg.LogFileCompress {
		t.Errorf("file defaults wrong: %+v", cfg)
	}
}

func TestLoadConfigFromEnv_OverrideAll(t *testing.T) {
	t.Setenv("OBSERVABILITY_DRIVER", "otlp")
	t.Setenv("OBSERVABILITY_LOG_LEVEL", "warn")
	t.Setenv("OBSERVABILITY_TRACE_SAMPLE_RATE", "0.25")
	t.Setenv("DEPLOYMENT_ENVIRONMENT", "prod")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	t.Setenv("AWS_REGION", "ap-northeast-1")
	t.Setenv("OBSERVABILITY_CLOUDWATCH_LOG_GROUP", "/svc/my-app")

	cfg := observability.LoadConfigFromEnv()
	if cfg.Driver != "otlp" || cfg.LogLevel != slog.LevelWarn || cfg.TraceSampleRate != 0.25 ||
		cfg.Environment != "prod" || cfg.OTLPEndpoint != "otel:4317" ||
		cfg.OTLPProtocol != "http" || !cfg.OTLPInsecure || cfg.AWSRegion != "ap-northeast-1" ||
		cfg.CloudWatchLogGroup != "/svc/my-app" {
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
				Driver:      observability.DriverStdout,
			},
		},
		{
			name: "missing service name",
			cfg: observability.Config{
				Driver: observability.DriverStdout,
			},
			wantErr: true,
		},
		{
			name: "unknown driver",
			cfg: observability.Config{
				ServiceName: "svc",
				Driver:      "voodoo",
			},
			wantErr: true,
		},
		{
			name: "otlp without endpoint",
			cfg: observability.Config{
				ServiceName: "svc",
				Driver:      observability.DriverOTLP,
			},
			wantErr: true,
		},
		{
			name: "file without path",
			cfg: observability.Config{
				ServiceName: "svc",
				Driver:      observability.DriverFile,
			},
			wantErr: true,
		},
		{
			name: "file with path ok",
			cfg: observability.Config{
				ServiceName: "svc",
				Driver:      observability.DriverFile,
				LogFilePath: "/tmp/log.log",
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
