package auth

// ClaimString returns a string claim from p.Claims, or ("", false) when absent.
func ClaimString(p *Principal, key string) (string, bool) {
	if p == nil || p.Claims == nil {
		return "", false
	}
	v, ok := p.Claims[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}
