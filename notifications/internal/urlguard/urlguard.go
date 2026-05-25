// Package urlguard validates outbound destination URLs for the
// webhook-style notification drivers (webhook/slack/discord) to defend
// against Server-Side Request Forgery (SSRF).
//
// Destination URLs in these drivers are attacker-influenced (a user can
// supply their own notification route), so before a request is issued the
// scheme is restricted and the target host is checked against a deny-list
// of private/loopback/link-local/metadata ranges. Both literal IP hosts
// and names that resolve (via DNS) to a blocked range are refused, which
// also closes DNS-rebinding-style tricks at send time.
package urlguard

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// ErrBlockedURL is returned (wrapped) when a URL fails validation: a
// disallowed scheme, an unresolvable host, or a host that points at a
// blocked address range.
var ErrBlockedURL = errors.New("notifications/urlguard: destination URL blocked by SSRF policy")

// blockedPrefixes enumerates the address ranges a notification target is
// never allowed to reach. IPv4-mapped IPv6 addresses are normalised with
// Addr.Unmap before matching so e.g. ::ffff:127.0.0.1 is also caught.
var blockedPrefixes = []netip.Prefix{
	// IPv4
	netip.MustParsePrefix("0.0.0.0/8"),      // "this host", incl. 0.0.0.0
	netip.MustParsePrefix("10.0.0.0/8"),     // private
	netip.MustParsePrefix("127.0.0.0/8"),    // loopback
	netip.MustParsePrefix("169.254.0.0/16"), // link-local, incl. 169.254.169.254 metadata
	netip.MustParsePrefix("172.16.0.0/12"),  // private
	netip.MustParsePrefix("192.168.0.0/16"), // private
	// IPv6
	netip.MustParsePrefix("::1/128"),   // loopback
	netip.MustParsePrefix("::/128"),    // unspecified
	netip.MustParsePrefix("fc00::/7"),  // unique-local
	netip.MustParsePrefix("fe80::/10"), // link-local
}

// resolver is overridable in tests; defaults to net.LookupIP.
var resolver = net.LookupIP

// Validate parses rawURL, enforces an https/http scheme, and verifies the
// host does not point at a private/loopback/link-local/metadata range
// (either as a literal IP or via DNS resolution). On success it returns
// the parsed URL; on failure it returns an error wrapping ErrBlockedURL.
//
// SECURITY: callers must validate the *final* URL actually requested. The
// drivers do not follow redirects on the injected HTTP client by default;
// if redirect-following is ever enabled, the redirect target must be
// re-validated through this function, otherwise a 30x to an internal host
// would bypass this check.
func Validate(rawURL string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("%w: parse: %v", ErrBlockedURL, err)
	}
	scheme := strings.ToLower(u.Scheme)
	// http is permitted in addition to https so on-prem Slack/webhook
	// endpoints work; private targets are still blocked below regardless
	// of scheme. Everything else (file:, gopher:, ftp:, ...) is refused.
	if scheme != "https" && scheme != "http" {
		return nil, fmt.Errorf("%w: scheme %q not allowed (only http/https)", ErrBlockedURL, u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("%w: missing host", ErrBlockedURL)
	}

	// Literal IP host: check directly without DNS.
	if addr, err := netip.ParseAddr(host); err == nil {
		if isBlocked(addr) {
			return nil, fmt.Errorf("%w: host %s is in a blocked range", ErrBlockedURL, host)
		}
		return u, nil
	}

	// Hostname: resolve and ensure every resolved address is allowed.
	ips, err := resolver(host)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve %q: %v", ErrBlockedURL, host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("%w: %q resolved to no addresses", ErrBlockedURL, host)
	}
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			return nil, fmt.Errorf("%w: %q resolved to an unparseable address", ErrBlockedURL, host)
		}
		if isBlocked(addr) {
			return nil, fmt.Errorf("%w: %q resolves to blocked address %s", ErrBlockedURL, host, addr)
		}
	}
	return u, nil
}

func isBlocked(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() {
		return true
	}
	if addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsUnspecified() {
		return true
	}
	for _, p := range blockedPrefixes {
		// Compare in matching families; Prefix.Contains handles this.
		if p.Contains(addr) {
			return true
		}
	}
	return false
}
