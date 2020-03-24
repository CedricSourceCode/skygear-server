package http

import (
	"net"
	"net/http"
	"time"

	"golang.org/x/net/publicsuffix"
)

type CookieConfiguration struct {
	Name   string
	Path   string
	Domain string
	Secure bool
	MaxAge *int
}

func (c *CookieConfiguration) WriteTo(rw http.ResponseWriter, value string) {
	cookie := &http.Cookie{
		Name:     c.Name,
		Path:     c.Path,
		Domain:   c.Domain,
		HttpOnly: true,
		Secure:   c.Secure,
		SameSite: http.SameSiteLaxMode,
	}

	cookie.Value = value
	if c.MaxAge != nil {
		cookie.MaxAge = *c.MaxAge
	}

	UpdateCookie(rw, cookie)
}

func (c *CookieConfiguration) Clear(rw http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     c.Name,
		Path:     c.Path,
		Domain:   c.Domain,
		HttpOnly: true,
		Secure:   c.Secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
	}

	UpdateCookie(rw, cookie)
}

// Cookie names
const (
	CookieNameSession = "session"
	// nolint: gosec
	CookieNameMFABearerToken = "mfa_bearer_token"
)

func UpdateCookie(w http.ResponseWriter, cookie *http.Cookie) {
	header := w.Header()
	resp := http.Response{Header: header}
	cookies := resp.Cookies()
	updated := false
	for i, c := range cookies {
		if c.Name == cookie.Name && c.Domain == cookie.Domain && c.Path == cookie.Path {
			cookies[i] = cookie
			updated = true
		}
	}
	if !updated {
		cookies = append(cookies, cookie)
	}
	setCookies := make([]string, len(cookies))
	for i, c := range cookies {
		setCookies[i] = c.String()
	}
	header["Set-Cookie"] = setCookies
}

// CookieDomainFromETLDPlusOneWithoutPort derives host from r.
// If host has port, the port is removed.
// If ETLD+1 cannot be derived, an empty string is returned.
// The return value never have port.
func CookieDomainFromETLDPlusOneWithoutPort(host string) string {
	// Trim the port if it is present.
	// We have to trim the port first.
	// Passing host:port to EffectiveTLDPlusOne confuses it.
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return ""
	}
	return host
}
