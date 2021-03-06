package handler

import (
	"errors"
	"time"

	"github.com/skygeario/skygear-server/pkg/auth/dependency/oauth"
	"github.com/skygeario/skygear-server/pkg/core/uuid"
)

func checkAuthorization(
	authzs oauth.AuthorizationStore,
	timestamp time.Time,
	appID string,
	clientID string,
	userID string,
	scopes []string,
) (*oauth.Authorization, error) {
	authz, err := authzs.Get(userID, clientID)
	if err == nil && authz.IsAuthorized(scopes) {
		return authz, nil
	} else if err != nil && !errors.Is(err, oauth.ErrAuthorizationNotFound) {
		return nil, err
	}

	// Authorization of requested scopes not granted, requesting consent.
	// TODO(oauth): request consent, for now just always implicitly grant scopes.
	if authz == nil {
		authz = &oauth.Authorization{
			ID:        uuid.New(),
			AppID:     appID,
			ClientID:  clientID,
			UserID:    userID,
			CreatedAt: timestamp,
			UpdatedAt: timestamp,
			Scopes:    scopes,
		}
		err = authzs.Create(authz)
		if err != nil {
			return nil, err
		}
	} else {
		authz = authz.WithScopesAdded(scopes)
		authz.UpdatedAt = timestamp
		err = authzs.UpdateScopes(authz)
		if err != nil {
			return nil, err
		}
	}

	return authz, nil
}
