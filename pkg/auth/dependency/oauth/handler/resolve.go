package handler

import (
	"net/url"

	"github.com/skygeario/skygear-server/pkg/auth/dependency/oauth/protocol"
	"github.com/skygeario/skygear-server/pkg/core/config"
)

type oauthRequest interface {
	ClientID() string
	RedirectURI() string
}

func resolveClient(clients []config.OAuthClientConfiguration, r oauthRequest) config.OAuthClientConfiguration {
	for _, c := range clients {
		if c.ClientID() == r.ClientID() {
			return c
		}
	}
	return nil
}

func parseRedirectURI(client config.OAuthClientConfiguration, r oauthRequest) (*url.URL, protocol.ErrorResponse) {
	allowedURIs := client.RedirectURIs()
	redirectURIString := r.RedirectURI()
	if len(allowedURIs) == 1 && redirectURIString == "" {
		// Redirect URI is default to the only allowed URI if possible.
		redirectURIString = allowedURIs[0]
	}

	redirectURI, err := url.Parse(redirectURIString)
	if err != nil {
		return nil, protocol.NewErrorResponse("invalid_request", "invalid redirect URI")
	}

	allowed := false
	for _, u := range allowedURIs {
		if u == redirectURIString {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, protocol.NewErrorResponse("invalid_request", "redirect URI is not allowed")
	}

	return redirectURI, nil
}
