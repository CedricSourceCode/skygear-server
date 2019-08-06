package sso

import (
	"github.com/skygeario/skygear-server/pkg/auth/dependency/hook"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/principal"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/principal/oauth"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/principal/password"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/sso"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/userprofile"
	"github.com/skygeario/skygear-server/pkg/auth/event"
	signUpHandler "github.com/skygeario/skygear-server/pkg/auth/handler"
	"github.com/skygeario/skygear-server/pkg/auth/model"
	"github.com/skygeario/skygear-server/pkg/auth/task"
	"github.com/skygeario/skygear-server/pkg/core/async"
	"github.com/skygeario/skygear-server/pkg/core/auth/authinfo"
	"github.com/skygeario/skygear-server/pkg/core/auth/authtoken"
	"github.com/skygeario/skygear-server/pkg/core/auth/metadata"
	"github.com/skygeario/skygear-server/pkg/core/skydb"
	"github.com/skygeario/skygear-server/pkg/core/skyerr"
)

type respHandler struct {
	TokenStore           authtoken.Store
	AuthInfoStore        authinfo.Store
	OAuthAuthProvider    oauth.Provider
	PasswordAuthProvider password.Provider
	IdentityProvider     principal.IdentityProvider
	UserProfileStore     userprofile.Store
	HookProvider         hook.Provider
	TaskQueue            async.Queue
	WelcomeEmailEnabled  bool
}

func (h respHandler) loginActionResp(oauthAuthInfo sso.AuthInfo, loginState sso.LoginState) (resp interface{}, err error) {
	// action => login
	var info authinfo.AuthInfo
	createNewUser, principal, err := h.handleLogin(oauthAuthInfo, &info, loginState)
	if err != nil {
		return
	}

	// Create empty user profile or get the existing one
	var userProfile userprofile.UserProfile
	emptyProfile := map[string]interface{}{}
	if createNewUser {
		userProfile, err = h.UserProfileStore.CreateUserProfile(info.ID, emptyProfile)
	} else {
		userProfile, err = h.UserProfileStore.GetUserProfile(info.ID)
	}
	if err != nil {
		// TODO:
		// return proper error
		err = skyerr.NewError(skyerr.UnexpectedError, "Unable to save user profile")
		return
	}

	user := model.NewUser(info, userProfile)
	identity := model.NewIdentity(h.IdentityProvider, principal)

	if createNewUser {
		err = h.HookProvider.DispatchEvent(
			event.UserCreateEvent{
				User:       user,
				Identities: []model.Identity{identity},
			},
			&user,
		)
		if err != nil {
			return
		}
	}

	// Create auth token
	var token authtoken.Token
	token, err = h.TokenStore.NewToken(info.ID, principal.ID)
	if err != nil {
		panic(err)
	}
	if err = h.TokenStore.Put(&token); err != nil {
		panic(err)
	}

	var sessionCreateReason event.SessionCreateReason
	if createNewUser {
		sessionCreateReason = event.SessionCreateReasonSignup
	} else {
		sessionCreateReason = event.SessionCreateReasonLogin
	}
	err = h.HookProvider.DispatchEvent(
		event.SessionCreateEvent{
			Reason:   sessionCreateReason,
			User:     user,
			Identity: identity,
		},
		&user,
	)
	if err != nil {
		return
	}

	// Reload auth info, in case before hook handler mutated it
	if err = h.AuthInfoStore.GetAuth(principal.UserID, &info); err != nil {
		return
	}

	// Update the activity time of user (return old activity time for usefulness)
	now := timeNow()
	info.LastLoginAt = &now
	info.LastSeenAt = &now
	if err = h.AuthInfoStore.UpdateAuth(&info); err != nil {
		err = skyerr.MakeError(err)
		return
	}

	resp = model.NewAuthResponse(user, identity, token.AccessToken)

	if createNewUser &&
		h.WelcomeEmailEnabled &&
		oauthAuthInfo.ProviderUserInfo.Email != "" &&
		h.TaskQueue != nil {
		h.TaskQueue.Enqueue(task.WelcomeEmailSendTaskName, task.WelcomeEmailSendTaskParam{
			Email: oauthAuthInfo.ProviderUserInfo.Email,
			User:  user,
		}, nil)
	}

	return
}

func (h respHandler) linkActionResp(oauthAuthInfo sso.AuthInfo, linkState sso.LinkState) (resp interface{}, err error) {
	// action => link
	// We only need to check if we can find such principal.
	// If such principal exists, it does not matter whether the principal
	// is associated with the user.
	// We do not allow the same provider user to be associated with an user
	// more than once.
	_, err = h.OAuthAuthProvider.GetPrincipalByProvider(oauth.GetByProviderOptions{
		ProviderType:   string(oauthAuthInfo.ProviderConfig.Type),
		ProviderKeys:   oauth.ProviderKeysFromProviderConfig(oauthAuthInfo.ProviderConfig),
		ProviderUserID: oauthAuthInfo.ProviderUserInfo.ID,
	})
	if err == nil {
		err = skyerr.NewError(skyerr.InvalidArgument, "the provider user is already linked")
		return resp, err
	}

	if err != skydb.ErrUserNotFound {
		// some other error
		return resp, err
	}

	var info authinfo.AuthInfo
	if err = h.AuthInfoStore.GetAuth(linkState.UserID, &info); err != nil {
		err = skyerr.NewError(skyerr.ResourceNotFound, "user not found")
		return resp, err
	}

	var principal *oauth.Principal
	principal, err = h.createPrincipalByOAuthInfo(info.ID, oauthAuthInfo)
	if err != nil {
		return resp, err
	}

	var userProfile userprofile.UserProfile
	userProfile, err = h.UserProfileStore.GetUserProfile(info.ID)
	if err != nil {
		return
	}

	user := model.NewUser(info, userProfile)
	identity := model.NewIdentity(h.IdentityProvider, principal)
	err = h.HookProvider.DispatchEvent(
		event.IdentityCreateEvent{
			User:     user,
			Identity: identity,
		},
		&user,
	)
	if err != nil {
		return
	}

	resp = map[string]string{}
	return
}

func (h respHandler) handleLogin(
	oauthAuthInfo sso.AuthInfo,
	info *authinfo.AuthInfo,
	loginState sso.LoginState,
) (createNewUser bool, oauthPrincipal *oauth.Principal, err error) {
	oauthPrincipal, err = h.findExistingOAuthPrincipal(oauthAuthInfo)
	if err != nil {
		return
	}

	now := timeNow()

	// Two func that closes over the arguments and the return value
	// and need to be reused.

	// populateInfo sets the argument info to non-nil value
	populateInfo := func(userID string) {
		if e := h.AuthInfoStore.GetAuth(userID, info); e != nil {
			if e == skydb.ErrUserNotFound {
				err = skyerr.NewError(skyerr.ResourceNotFound, "User not found")
				return
			}
			err = skyerr.MakeError(e)
			return
		}
	}

	// createFunc creates a new user.
	createFunc := func() {
		createNewUser = true
		// if there is no existed user
		// signup a new user
		*info = authinfo.NewAuthInfo()
		info.LastLoginAt = &now

		// Create AuthInfo
		if e := h.AuthInfoStore.CreateAuth(info); e != nil {
			if e == skydb.ErrUserDuplicated {
				err = signUpHandler.ErrUserDuplicated
				return
			}
			// TODO:
			// return proper error
			err = skyerr.NewError(skyerr.UnexpectedError, "Unable to save auth info")
			return
		}

		oauthPrincipal, err = h.createPrincipalByOAuthInfo(info.ID, oauthAuthInfo)
		if err != nil {
			return
		}
	}

	// Case: OAuth principal was found
	// => Simple update case
	// We do not need to consider password principal
	if oauthPrincipal != nil {
		oauthPrincipal.AccessTokenResp = oauthAuthInfo.ProviderAccessTokenResp
		oauthPrincipal.UserProfile = oauthAuthInfo.ProviderRawProfile
		oauthPrincipal.UpdatedAt = &now
		if err = h.OAuthAuthProvider.UpdatePrincipal(oauthPrincipal); err != nil {
			err = skyerr.MakeError(err)
			return
		}
		populateInfo(oauthPrincipal.UserID)
		// Always return here because we are done with this case.
		return
	}

	// Case: OAuth principal was not found
	// We need to consider password principal
	passwordPrincipal, err := h.findExistingPasswordPrincipal(oauthAuthInfo, loginState.MergeRealm)
	if err != nil {
		return
	}

	// Case: OAuth principal was not found and Password principal was not found
	// => Simple create case
	if passwordPrincipal == nil {
		createFunc()
		return
	}

	// Case: OAuth principal was not found and Password principal was found
	// => Complex case
	switch loginState.OnUserDuplicate {
	case sso.OnUserDuplicateAbort:
		err = skyerr.NewError(skyerr.Duplicated, "Aborted due to duplicate user")
	case sso.OnUserDuplicateCreate:
		createFunc()
	case sso.OnUserDuplicateMerge:
		// Associate the provider to the existing user
		oauthPrincipal, err = h.createPrincipalByOAuthInfo(
			passwordPrincipal.UserID,
			oauthAuthInfo,
		)
		if err != nil {
			return
		}
		populateInfo(passwordPrincipal.UserID)
	}

	return
}

func (h respHandler) findExistingOAuthPrincipal(oauthAuthInfo sso.AuthInfo) (*oauth.Principal, error) {
	// Find oauth principal from by (provider_id, provider_user_id)
	principal, err := h.OAuthAuthProvider.GetPrincipalByProvider(oauth.GetByProviderOptions{
		ProviderType:   string(oauthAuthInfo.ProviderConfig.Type),
		ProviderKeys:   oauth.ProviderKeysFromProviderConfig(oauthAuthInfo.ProviderConfig),
		ProviderUserID: oauthAuthInfo.ProviderUserInfo.ID,
	})
	if err == skydb.ErrUserNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return principal, nil
}

func (h respHandler) findExistingPasswordPrincipal(oauthAuthInfo sso.AuthInfo, mergeRealm string) (*password.Principal, error) {
	// Find password principal by provider primary email
	email := oauthAuthInfo.ProviderUserInfo.Email
	if email == "" {
		return nil, nil
	}
	passwordPrincipal := password.Principal{}
	err := h.PasswordAuthProvider.GetPrincipalByLoginIDWithRealm("", email, mergeRealm, &passwordPrincipal)
	if err == skydb.ErrUserNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !h.PasswordAuthProvider.CheckLoginIDKeyType(passwordPrincipal.LoginIDKey, metadata.Email) {
		return nil, nil
	}
	return &passwordPrincipal, nil
}

func (h respHandler) createPrincipalByOAuthInfo(userID string, oauthAuthInfo sso.AuthInfo) (*oauth.Principal, error) {
	now := timeNow()
	providerKeys := oauth.ProviderKeysFromProviderConfig(oauthAuthInfo.ProviderConfig)
	principal := oauth.NewPrincipal(providerKeys)
	principal.UserID = userID
	principal.ProviderType = string(oauthAuthInfo.ProviderConfig.Type)
	principal.ProviderUserID = oauthAuthInfo.ProviderUserInfo.ID
	principal.AccessTokenResp = oauthAuthInfo.ProviderAccessTokenResp
	principal.UserProfile = oauthAuthInfo.ProviderRawProfile
	principal.CreatedAt = &now
	principal.UpdatedAt = &now
	err := h.OAuthAuthProvider.CreatePrincipal(principal)
	return principal, err
}