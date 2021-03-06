package anonymous

import (
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/skygeario/skygear-server/pkg/core/jwt"
)

// nolint:gosec
const RequestTokenType = "vnd.skygear.auth.anonymous-request"

type RequestAction string

const (
	RequestActionAuth    RequestAction = "auth"
	RequestActionPromote RequestAction = "promote"
)

type Request struct {
	jwt.StandardClaims
	Key       jwk.Key       `json:"-"`
	Challenge string        `json:"challenge"`
	Action    RequestAction `json:"action"`
}
