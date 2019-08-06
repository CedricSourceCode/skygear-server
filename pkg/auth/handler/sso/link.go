package sso

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/hook"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/principal"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/principal/oauth"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/sso"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/userprofile"
	"github.com/skygeario/skygear-server/pkg/core/skyerr"

	"github.com/skygeario/skygear-server/pkg/auth"
	coreAuth "github.com/skygeario/skygear-server/pkg/core/auth"
	"github.com/skygeario/skygear-server/pkg/core/auth/authinfo"
	"github.com/skygeario/skygear-server/pkg/core/auth/authz"
	"github.com/skygeario/skygear-server/pkg/core/auth/authz/policy"
	"github.com/skygeario/skygear-server/pkg/core/config"
	"github.com/skygeario/skygear-server/pkg/core/db"
	"github.com/skygeario/skygear-server/pkg/core/handler"
	"github.com/skygeario/skygear-server/pkg/core/inject"
	"github.com/skygeario/skygear-server/pkg/core/server"
)

func AttachLinkHandler(
	server *server.Server,
	authDependency auth.DependencyMap,
) *server.Server {
	server.Handle("/sso/{provider}/link", &LinkHandlerFactory{
		Dependency: authDependency,
	}).Methods("OPTIONS", "POST")
	return server
}

type LinkHandlerFactory struct {
	Dependency auth.DependencyMap
}

func (f LinkHandlerFactory) NewHandler(request *http.Request) http.Handler {
	h := &LinkHandler{}
	inject.DefaultRequestInject(h, f.Dependency, request)
	vars := mux.Vars(request)
	h.ProviderID = vars["provider"]
	h.Provider = h.ProviderFactory.NewProvider(h.ProviderID)
	return handler.APIHandlerToHandler(hook.WrapHandler(h.HookProvider, h), h.TxContext)
}

func (f LinkHandlerFactory) ProvideAuthzPolicy() authz.Policy {
	return policy.AllOf(
		authz.PolicyFunc(policy.DenyNoAccessKey),
		authz.PolicyFunc(policy.RequireAuthenticated),
		authz.PolicyFunc(policy.DenyDisabledUser),
	)
}

// LinkRequestPayload login handler request payload
type LinkRequestPayload struct {
	AccessToken string `json:"access_token"`
}

// Validate request payload
func (p LinkRequestPayload) Validate() error {
	if p.AccessToken == "" {
		return skyerr.NewInvalidArgument("empty access token", []string{"access_token"})
	}

	return nil
}

// LinkHandler decodes code response and fetch access token from provider.
//
// curl \
//   -X POST \
//   -H "Content-Type: application/json" \
//   -H "X-Skygear-Api-Key: API_KEY" \
//   -d @- \
//   http://localhost:3000/sso/<provider>/link \
// <<EOF
// {
//     "token_response": {
//       "access_token": "<access_token>"
//     }
// }
// EOF
//
// {
//     "result": {}
// }
//
type LinkHandler struct {
	TxContext          db.TxContext               `dependency:"TxContext"`
	AuthContext        coreAuth.ContextGetter     `dependency:"AuthContextGetter"`
	OAuthAuthProvider  oauth.Provider             `dependency:"OAuthAuthProvider"`
	IdentityProvider   principal.IdentityProvider `dependency:"IdentityProvider"`
	AuthInfoStore      authinfo.Store             `dependency:"AuthInfoStore"`
	UserProfileStore   userprofile.Store          `dependency:"UserProfileStore"`
	HookProvider       hook.Provider              `dependency:"HookProvider"`
	ProviderFactory    *sso.ProviderFactory       `dependency:"SSOProviderFactory"`
	OAuthConfiguration config.OAuthConfiguration  `dependency:"OAuthConfiguration"`
	Provider           sso.OAuthProvider
	ProviderID         string
}

func (h LinkHandler) WithTx() bool {
	return true
}

func (h LinkHandler) DecodeRequest(request *http.Request) (handler.RequestPayload, error) {
	payload := LinkRequestPayload{}
	err := json.NewDecoder(request.Body).Decode(&payload)
	if err != nil {
		return payload, err
	}
	return payload, nil
}

func (h LinkHandler) Handle(req interface{}) (resp interface{}, err error) {
	if !h.OAuthConfiguration.ExternalAccessTokenFlowEnabled {
		err = skyerr.NewError(skyerr.UndefinedOperation, "External access token flow is disabled")
		return
	}

	provider, ok := h.Provider.(sso.ExternalAccessTokenFlowProvider)
	if !ok {
		err = skyerr.NewInvalidArgument("Provider is not supported", []string{h.ProviderID})
		return
	}

	userID := h.AuthContext.AuthInfo().ID
	payload := req.(LinkRequestPayload)

	linkState := sso.LinkState{
		UserID: userID,
	}
	oauthAuthInfo, err := provider.ExternalAccessTokenGetAuthInfo(sso.NewBearerAccessTokenResp(payload.AccessToken))
	if err != nil {
		return
	}

	handler := respHandler{
		AuthInfoStore:     h.AuthInfoStore,
		OAuthAuthProvider: h.OAuthAuthProvider,
		IdentityProvider:  h.IdentityProvider,
		UserProfileStore:  h.UserProfileStore,
		HookProvider:      h.HookProvider,
	}
	resp, err = handler.linkActionResp(oauthAuthInfo, linkState)

	return
}