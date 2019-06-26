package userverify

import (
	"fmt"

	"github.com/skygeario/skygear-server/pkg/core/auth/authinfo"
	"github.com/skygeario/skygear-server/pkg/core/config"
)

type AutoUpdateUserVerifyFunc func(*authinfo.AuthInfo)

func CreateAutoUpdateUserVerifyfunc(tConfig config.TenantConfiguration) AutoUpdateUserVerifyFunc {
	if !tConfig.UserConfig.UserVerification.AutoUpdate {
		return nil
	}

	switch tConfig.UserConfig.UserVerification.Criteria {
	case "all":
		return func(authInfo *authinfo.AuthInfo) {
			allVerified := true
			for _, keyConfig := range tConfig.UserConfig.UserVerification.Keys {
				if !authInfo.VerifyInfo[keyConfig.Key] {
					allVerified = false
					break
				}
			}

			authInfo.Verified = allVerified
		}
	case "any":
		return func(authInfo *authinfo.AuthInfo) {
			for _, keyConfig := range tConfig.UserConfig.UserVerification.Keys {
				if authInfo.VerifyInfo[keyConfig.Key] {
					authInfo.Verified = true
					return
				}
			}

			authInfo.Verified = false
		}
	default:
		panic(fmt.Errorf("unexpected verify criteria `%s`", tConfig.UserConfig.UserVerification.Criteria))
	}
}