package userverify

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/provider/password"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/userverify"

	"github.com/skygeario/skygear-server/pkg/core/auth"
	"github.com/skygeario/skygear-server/pkg/core/auth/authinfo"
	"github.com/skygeario/skygear-server/pkg/core/config"
	"github.com/skygeario/skygear-server/pkg/core/db"
	"github.com/skygeario/skygear-server/pkg/core/handler"
	. "github.com/skygeario/skygear-server/pkg/core/skytest"
	. "github.com/smartystreets/goconvey/convey"
)

func TestForgotPasswordResetHandler(t *testing.T) {
	realTime := timeNow
	timeNow = func() time.Time { return time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC) }
	defer func() {
		timeNow = realTime
	}()

	Convey("Test VerifyCodeHandler", t, func() {
		vh := &VerifyCodeHandler{}
		logger, _ := test.NewNullLogger()
		vh.Logger = logrus.NewEntry(logger)
		vh.TxContext = db.NewMockTxContext()
		vh.AuthContext = auth.NewMockContextGetterWithUnverifiedUser(map[string]bool{
			"faseng.cat.id@example.com": false,
		})

		zero := 0
		one := 1
		loginIDsKeys := map[string]config.LoginIDKeyConfiguration{
			"email": config.LoginIDKeyConfiguration{Minimum: &zero, Maximum: &one},
		}
		vh.PasswordAuthProvider = password.NewMockProviderWithPrincipalMap(
			loginIDsKeys,
			[]string{password.DefaultRealm},
			map[string]password.Principal{
				"faseng1": password.Principal{
					ID:             "id1",
					UserID:         "faseng.cat.id",
					LoginIDKey:     "email",
					LoginID:        "faseng.cat.id@example.com",
					Realm:          "default",
					HashedPassword: []byte("$2a$10$/jm/S1sY6ldfL6UZljlJdOAdJojsJfkjg/pqK47Q8WmOLE19tGWQi"), // 123456
				},
				"faseng2": password.Principal{
					ID:             "id2",
					UserID:         "faseng.cat.id",
					LoginIDKey:     "phone",
					LoginID:        "+85299999999",
					Realm:          "default",
					HashedPassword: []byte("$2a$10$/jm/S1sY6ldfL6UZljlJdOAdJojsJfkjg/pqK47Q8WmOLE19tGWQi"), // 123456
				},
				"chima1": password.Principal{
					ID:             "id2",
					UserID:         "chima.cat.id",
					LoginIDKey:     "email",
					LoginID:        "chima.cat.id@example.com",
					Realm:          "default",
					HashedPassword: []byte("$2a$10$/jm/S1sY6ldfL6UZljlJdOAdJojsJfkjg/pqK47Q8WmOLE19tGWQi"), // 123456
				},
			},
		)

		authInfo := authinfo.AuthInfo{
			ID: "faseng.cat.id",
		}
		authInfoStore := authinfo.NewMockStoreWithAuthInfoMap(
			map[string]authinfo.AuthInfo{
				"faseng.cat.id": authInfo,
			},
		)
		vh.AuthInfoStore = authInfoStore

		verifyConfig := config.UserVerificationConfiguration{
			LoginIDKeys: map[string]config.UserVerificationKeyConfiguration{
				"email": config.UserVerificationKeyConfiguration{
					Expiry: 12 * 60 * 60,
				},
			},
		}
		vh.UserVerificationProvider = userverify.NewProvider(
			nil,
			&userverify.MockStore{
				CodeByID: map[string]userverify.VerifyCode{
					"code": userverify.VerifyCode{
						ID:         "code",
						UserID:     "faseng.cat.id",
						LoginIDKey: "email",
						LoginID:    "faseng.cat.id@example.com",
						Code:       "code1",
						Consumed:   false,
						CreatedAt:  timeNow(),
					},
					"code-old": userverify.VerifyCode{
						ID:         "code-old",
						UserID:     "faseng.cat.id",
						LoginIDKey: "email",
						LoginID:    "faseng.cat.id@example.com",
						Code:       "code2",
						Consumed:   false,
						CreatedAt:  timeNow().Add(-time.Duration(24) * time.Hour),
					},
					"code-someoneelse": userverify.VerifyCode{
						ID:         "code1",
						UserID:     "chima.cat.id",
						LoginIDKey: "email",
						LoginID:    "faseng.cat.id@example.com",
						Code:       "code3",
						Consumed:   false,
						CreatedAt:  timeNow(),
					},
					"code-consumed": userverify.VerifyCode{
						ID:         "code-consumed",
						UserID:     "faseng.cat.id",
						LoginIDKey: "email",
						LoginID:    "faseng.cat.id@example.com",
						Code:       "code4",
						Consumed:   true,
						CreatedAt:  timeNow(),
					},
				},
			},
			verifyConfig,
		)

		Convey("verify with correct code and auto update", func() {
			req, _ := http.NewRequest("POST", "", strings.NewReader(`{
				"code": "code1"
			}`))
			resp := httptest.NewRecorder()
			h := handler.APIHandlerToHandler(vh, vh.TxContext)
			h.ServeHTTP(resp, req)
			So(resp.Body.Bytes(), ShouldEqualJSON, `{
				"result": "OK"
			}`)
			So(authInfoStore.AuthInfoMap["faseng.cat.id"].Verified, ShouldBeTrue)
		})

		Convey("verify with correct code but not all verified", func() {
			verifyConfig.LoginIDKeys = map[string]config.UserVerificationKeyConfiguration{
				"email": config.UserVerificationKeyConfiguration{Expiry: 12 * 60 * 60},
				"phone": config.UserVerificationKeyConfiguration{Expiry: 12 * 60 * 60},
			}
			req, _ := http.NewRequest("POST", "", strings.NewReader(`{
				"code": "code1"
			}`))
			resp := httptest.NewRecorder()
			h := handler.APIHandlerToHandler(vh, vh.TxContext)
			h.ServeHTTP(resp, req)
			So(resp.Body.Bytes(), ShouldEqualJSON, `{
				"result": "OK"
			}`)
			So(authInfoStore.AuthInfoMap["faseng.cat.id"].Verified, ShouldBeFalse)
		})

		Convey("verify with expired code", func() {
			req, _ := http.NewRequest("POST", "", strings.NewReader(`{
				"code": "code2"
			}`))
			resp := httptest.NewRecorder()
			h := handler.APIHandlerToHandler(vh, vh.TxContext)
			h.ServeHTTP(resp, req)
			So(resp.Body.Bytes(), ShouldEqualJSON, `{
				"error": {
					"code": 108,
					"message": "the code has expired",
					"name": "InvalidArgument"
				}
			}`)
			So(authInfoStore.AuthInfoMap["faseng.cat.id"].Verified, ShouldBeFalse)
		})

		Convey("verify with someone else code", func() {
			req, _ := http.NewRequest("POST", "", strings.NewReader(`{
				"code": "code3"
			}`))
			resp := httptest.NewRecorder()
			h := handler.APIHandlerToHandler(vh, vh.TxContext)
			h.ServeHTTP(resp, req)
			So(resp.Body.Bytes(), ShouldEqualJSON, `{
				"error": {
					"code": 10000,
					"message": "code not found",
					"name": "UnexpectedError"
				}
			}`)
			So(authInfoStore.AuthInfoMap["faseng.cat.id"].Verified, ShouldBeFalse)
		})

		Convey("verify with consumed code", func() {
			req, _ := http.NewRequest("POST", "", strings.NewReader(`{
				"code": "code4"
			}`))
			resp := httptest.NewRecorder()
			h := handler.APIHandlerToHandler(vh, vh.TxContext)
			h.ServeHTTP(resp, req)
			So(resp.Body.Bytes(), ShouldEqualJSON, `{
				"error": {
					"code": 108,
					"message": "the code `+"`code4`"+` is not valid for user `+"`faseng.cat.id`"+`",
					"name": "InvalidArgument",
					"info": {
						"arguments": ["code"]
					}
				}
			}`)
			So(authInfoStore.AuthInfoMap["faseng.cat.id"].Verified, ShouldBeFalse)
		})

		Convey("verify with random code", func() {
			req, _ := http.NewRequest("POST", "", strings.NewReader(`{
				"code": "random"
			}`))
			resp := httptest.NewRecorder()
			h := handler.APIHandlerToHandler(vh, vh.TxContext)
			h.ServeHTTP(resp, req)
			So(resp.Body.Bytes(), ShouldEqualJSON, `{
				"error": {
					"code": 108,
					"message": "the code `+"`random`"+` is not valid for user `+"`faseng.cat.id`"+`",
					"name": "InvalidArgument",
					"info": {
						"arguments": ["code"]
					}
				}
			}`)
			So(authInfoStore.AuthInfoMap["faseng.cat.id"].Verified, ShouldBeFalse)
		})
	})
}
