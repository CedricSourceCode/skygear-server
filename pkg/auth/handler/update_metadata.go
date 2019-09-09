package handler

import (
	"net/http"

	"github.com/skygeario/skygear-server/pkg/auth/dependency/hook"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/principal"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/principal/password"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/userprofile"
	"github.com/skygeario/skygear-server/pkg/auth/event"

	"github.com/skygeario/skygear-server/pkg/auth"
	authModel "github.com/skygeario/skygear-server/pkg/auth/model"
	coreAuth "github.com/skygeario/skygear-server/pkg/core/auth"
	"github.com/skygeario/skygear-server/pkg/core/auth/authinfo"
	"github.com/skygeario/skygear-server/pkg/core/auth/authz"
	"github.com/skygeario/skygear-server/pkg/core/auth/authz/policy"
	"github.com/skygeario/skygear-server/pkg/core/db"
	"github.com/skygeario/skygear-server/pkg/core/handler"
	"github.com/skygeario/skygear-server/pkg/core/inject"
	"github.com/skygeario/skygear-server/pkg/core/server"
	"github.com/skygeario/skygear-server/pkg/core/skydb"
	"github.com/skygeario/skygear-server/pkg/core/skyerr"
)

func AttachUpdateMetadataHandler(
	server *server.Server,
	authDependency auth.DependencyMap,
) *server.Server {
	server.Handle("/update_metadata", &UpdateMetadataHandlerFactory{
		authDependency,
	}).Methods("OPTIONS", "POST")
	return server
}

type UpdateMetadataHandlerFactory struct {
	Dependency auth.DependencyMap
}

func (f UpdateMetadataHandlerFactory) NewHandler(request *http.Request) http.Handler {
	h := &UpdateMetadataHandler{}
	inject.DefaultRequestInject(h, f.Dependency, request)
	return handler.RequireAuthz(handler.APIHandlerToHandler(hook.WrapHandler(h.HookProvider, h), h.TxContext), h.AuthContext, h)
}

type UpdateMetadataRequestPayload struct {
	UserID   string                 `json:"user_id"`
	Metadata map[string]interface{} `json:"metadata"`
}

// @JSONSchema
const UpdateMetadataRequestSchema = `
{
	"$id": "#UpdateMetadataRequest",
	"type": "object",
	"properties": {
		"user_id": { "type": "string" },
		"metadata": { "type": "object" }
	}
}
`

func (p UpdateMetadataRequestPayload) Validate() error {
	return nil
}

/*
	@Operation POST /update_metadata - Update metadata
		Changes metadata of current user.
		If master key is used as access key, other users can be specified.

		@Tag User
		@SecurityRequirement access_key
		@SecurityRequirement access_token

		@RequestBody
			Describe target user and new metadata.
			@JSONSchema {UpdateMetadataRequest}

		@Response 200
			User information with new metadata.
			@JSONSchema {UserResponse}

		@Callback user_update {UserUpdateEvent}
		@Callback user_sync {UserSyncEvent}
*/
type UpdateMetadataHandler struct {
	AuthContext          coreAuth.ContextGetter     `dependency:"AuthContextGetter"`
	AuthInfoStore        authinfo.Store             `dependency:"AuthInfoStore"`
	TxContext            db.TxContext               `dependency:"TxContext"`
	UserProfileStore     userprofile.Store          `dependency:"UserProfileStore"`
	PasswordAuthProvider password.Provider          `dependency:"PasswordAuthProvider"`
	IdentityProvider     principal.IdentityProvider `dependency:"IdentityProvider"`
	HookProvider         hook.Provider              `dependency:"HookProvider"`
}

func (h UpdateMetadataHandler) ProvideAuthzPolicy() authz.Policy {
	return policy.AnyOf(
		authz.PolicyFunc(policy.RequireMasterKey),
		policy.AllOf(
			authz.PolicyFunc(policy.DenyNoAccessKey),
			authz.PolicyFunc(policy.RequireAuthenticated),
			authz.PolicyFunc(policy.DenyDisabledUser),
		),
	)
}

func (h UpdateMetadataHandler) WithTx() bool {
	return true
}

func (h UpdateMetadataHandler) DecodeRequest(request *http.Request) (handler.RequestPayload, error) {
	payload := UpdateMetadataRequestPayload{}
	err := handler.DecodeJSONBody(request, &payload)
	return payload, err
}

func (h UpdateMetadataHandler) Handle(req interface{}) (resp interface{}, err error) {
	payload := req.(UpdateMetadataRequestPayload)
	accessKey := h.AuthContext.AccessKey()

	var targetUserID string
	if accessKey.IsMasterKey() {
		if payload.UserID == "" {
			err = skyerr.NewInvalidArgument("empty user_id", []string{"user_id"})
			return
		}
		targetUserID = payload.UserID
	} else {
		if payload.UserID != "" {
			err = skyerr.NewError(skyerr.PermissionDenied, "must not specify user_id")
			return
		}
		targetUserID = h.AuthContext.AuthInfo().ID
	}

	newMetadata := payload.Metadata

	authInfo := authinfo.AuthInfo{}
	if e := h.AuthInfoStore.GetAuth(targetUserID, &authInfo); e != nil {
		if err == skydb.ErrUserNotFound {
			err = skyerr.NewError(skyerr.ResourceNotFound, "User not found")
			return
		}
		// TODO: more error handling here if necessary
		err = skyerr.NewResourceFetchFailureErr("auth_data", payload.UserID)
		return
	}

	var oldProfile, newProfile userprofile.UserProfile
	if oldProfile, err = h.UserProfileStore.GetUserProfile(authInfo.ID); err != nil {
		// TODO:
		// return proper error
		err = skyerr.NewError(skyerr.UnexpectedError, "Unable to get user profile")
		return
	}

	if newProfile, err = h.UserProfileStore.UpdateUserProfile(authInfo.ID, newMetadata); err != nil {
		// TODO:
		// return proper error
		err = skyerr.NewError(skyerr.UnexpectedError, "Unable to update user profile")
		return
	}

	oldUser := authModel.NewUser(authInfo, oldProfile)
	user := authModel.NewUser(authInfo, newProfile)

	err = h.HookProvider.DispatchEvent(
		event.UserUpdateEvent{
			Reason:   event.UserUpdateReasonUpdateMetadata,
			User:     oldUser,
			Metadata: &newProfile.Data,
		},
		&user,
	)
	if err != nil {
		return
	}

	resp = authModel.NewAuthResponseWithUser(user)

	return
}