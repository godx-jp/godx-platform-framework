package urlguard

import (
	"errors"
	"net"
	"testing"
)

func TestValidate_BlocksDangerousTargets(t *testing.T) {
	// Pin DNS so hostname cases are deterministic and offline.
	old := resolver
	resolver = func(host string) ([]net.IP, error) {
		switch host {
		case "localhost":
			return []net.IP{net.ParseIP("127.0.0.1")}, nil
		case "hooks.slack.com":
			return []net.IP{net.ParseIP("13.107.42.14")}, nil
		case "internal.rebind.example":
			return []net.IP{net.ParseIP("10.0.0.5")}, nil
		default:
			return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
		}
	}
	t.Cleanup(func() { resolver = old })

	blocked := []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://localhost/",
		"http://10.0.0.1/",
		"file:///etc/passwd",
		"gopher://127.0.0.1:6379/",
		"http://127.0.0.1:6379/",
		"http://192.168.1.1/admin",
		"http://[::1]/",
		"http://0.0.0.0/",
		"https://internal.rebind.example/",
		"https://[::ffff:127.0.0.1]/",
		"https://", // missing host
	}
	for _, raw := range blocked {
		t.Run("blocked/"+raw, func(t *testing.T) {
			if _, err := Validate(raw); err == nil {
				t.Fatalf("expected %q to be blocked, got nil error", raw)
			} else if !errors.Is(err, ErrBlockedURL) {
				t.Fatalf("expected ErrBlockedURL for %q, got %v", raw, err)
			}
		})
	}
}

func TestValidate_AllowsPublicHTTPS(t *testing.T) {
	old := resolver
	resolver = func(host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("13.107.42.14")}, nil
	}
	t.Cleanup(func() { resolver = old })

	allowed := []string{
		"https://hooks.slack.com/services/T000/B000/XXXX",
		"https://discord.com/api/webhooks/123/abc",
		"http://example.com/webhook", // http allowed for on-prem; public IP
	}
	for _, raw := range allowed {
		t.Run("allowed/"+raw, func(t *testing.T) {
			u, err := Validate(raw)
			if err != nil {
				t.Fatalf("expected %q to be allowed, got %v", raw, err)
			}
			if u == nil {
				t.Fatalf("expected parsed URL for %q", raw)
			}
		})
	}
}
