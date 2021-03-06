package handler

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"

	"github.com/skygeario/skygear-server/pkg/auth/dependency/oauth/protocol"
)

type AuthorizationResult interface {
	WriteResponse(rw http.ResponseWriter, r *http.Request)
	IsInternalError() bool
}

type (
	authorizationResultCode struct {
		RedirectURI  *url.URL
		ResponseMode string
		Response     protocol.AuthorizationResponse
	}
	authorizationResultError struct {
		RedirectURI   *url.URL
		ResponseMode  string
		InternalError bool
		Response      protocol.ErrorResponse
	}
	authorizationResultRequireAuthn struct {
		AuthenticateURI *url.URL
	}
)

func (a authorizationResultCode) WriteResponse(rw http.ResponseWriter, r *http.Request) {
	writeResponse(rw, r, a.RedirectURI, a.ResponseMode, a.Response)
}

func (a authorizationResultCode) IsInternalError() bool {
	return false
}

func (a authorizationResultError) WriteResponse(rw http.ResponseWriter, r *http.Request) {
	if a.RedirectURI != nil {
		writeResponse(rw, r, a.RedirectURI, a.ResponseMode, a.Response)
	} else {
		err := "Invalid OAuth authorization request:\n"
		keys := make([]string, 0, len(a.Response))
		for k := range a.Response {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i != 0 {
				err += "\n"
			}
			err += fmt.Sprintf("%s: %s", k, a.Response[k])
		}
		http.Error(rw, err, http.StatusBadRequest)
	}
}

func (a authorizationResultError) IsInternalError() bool {
	return a.InternalError
}

func (a authorizationResultRequireAuthn) WriteResponse(rw http.ResponseWriter, r *http.Request) {
	http.Redirect(rw, r, a.AuthenticateURI.String(), http.StatusFound)
}

func (a authorizationResultRequireAuthn) IsInternalError() bool {
	return false
}
