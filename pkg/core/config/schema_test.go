package config

import (
	"strings"
	"testing"

	"github.com/skygeario/skygear-server/pkg/core/validation"

	. "github.com/smartystreets/goconvey/convey"
)

func TestParseAppConfiguration(t *testing.T) {
	Convey("ParseAppConfiguration", t, func() {
		test := func(input string, errors ...string) {
			_, err := ParseAppConfiguration(strings.NewReader(input))
			if len(errors) == 0 {
				So(err, ShouldBeNil)
			} else {
				So(err, ShouldNotBeNil)
				So(validation.ErrorCauseStrings(err), ShouldResemble, errors)
			}
		}
		// Empty root
		test(
			`{}`,
			"/api_version: Required",
			"/asset: Required",
			"/authentication: Required",
			"/hook: Required",
			"/master_key: Required",
		)
		// Empty authentication
		test(`
			{
				"master_key": "master_key",
				"authentication": {},
				"hook": {}
			}`,
			"/api_version: Required",
			"/asset: Required",
			"/authentication/secret: Required",
			"/hook/secret: Required",
		)
		// Empty identity.login_id.keys
		test(`
			{
				"master_key": "master_key",
				"asset": {},
				"identity": {
					"login_id": {
						"keys": []
					}
				},
				"hook": {}
			}`,
			"/api_version: Required",
			"/asset/secret: Required",
			"/authentication: Required",
			"/hook/secret: Required",
			"/identity/login_id/keys: EntryAmount map[gte:1]",
		)
		// Invalid login id type
		test(`
			{
				"master_key": "master_key",
				"identity": {
					"login_id": {
						"keys": [
							{
								"key": "email",
								"type": "email"
							},
							{
								"key": "phone",
								"type": "phone"
							},
							{
								"key": "username",
								"type": "username"
							},
							{
								"key": "invalid",
								"type": "invalid"
							}
						],
						"types": {
							"email": {
								"case_sensitive": false,
								"block_plus_sign": false,
								"ignore_dot_sign": false
							},
							"username": {
								"block_reserved_usernames": true,
								"excluded_keywords": [ "skygear" ],
								"ascii_only": false,
								"case_sensitive": false
							},
							"phone": {}
						}
					}
				},
				"hook": {}
			}`,
			"/api_version: Required",
			"/asset: Required",
			"/authentication: Required",
			"/hook/secret: Required",
			"/identity/login_id/keys/3/type: Enum map[expected:[raw email phone username]]",
			"/identity/login_id/types/phone: ExtraEntry",
		)
		// Minimal valid example
		test(`
			{
				"api_version": "v2.1",
				"master_key": "master_key",
				"asset": {
					"secret": "assetsecret"
				},
				"authentication": {
					"secret": "authnsessionsecret"
				},
				"identity": {
					"login_id": {
						"keys": [
							{
								"key": "email",
								"type": "email"
							},
							{
								"key": "phone",
								"type": "phone"
							},
							{
								"key": "username",
								"type": "username"
							}
						]
					}
				},
				"hook": {
					"secret": "hooksecret"
				}
			}`,
		)
		// Session
		test(`
			{
				"api_version": "v2.1",
				"asset": {
					"secret": "assetsecret"
				},
				"session": {
					"lifetime": -1,
					"idle_timeout_enabled": "foobar",
					"idle_timeout": -1,
					"cookie_domain": 1,
					"cookie_non_persistent": 1
				},
				"master_key": "master_key",
				"authentication": {
					"secret": "authnsessionsecret"
				},
				"identity": {
					"login_id": {
						"keys": [
							{
								"key": "email",
								"type": "email"
							},
							{
								"key": "phone",
								"type": "phone"
							},
							{
								"key": "username",
								"type": "username"
							}
						]
					}
				},
				"hook": {
					"secret": "hooksecret"
				}
			}`,
			"/session/cookie_domain: Type map[expected:string]",
			"/session/cookie_non_persistent: Type map[expected:boolean]",
			"/session/idle_timeout: NumberRange map[gte:0]",
			"/session/idle_timeout_enabled: Type map[expected:boolean]",
			"/session/lifetime: NumberRange map[gte:0]",
		)
		// API Clients
		test(`
			{
				"api_version": "v2.1",
				"clients": [
					{
						"key": "web-app"
					}
				],
				"asset": {
					"secret": "assetsecret"
				},
				"master_key": "master_key",
				"authentication": {
					"secret": "authnsessionsecret"
				},
				"identity": {
					"login_id": {
						"keys": [
							{
								"key": "email",
								"type": "email"
							},
							{
								"key": "phone",
								"type": "phone"
							},
							{
								"key": "username",
								"type": "username"
							}
						]
					}
				},
				"hook": {
					"secret": "hooksecret"
				}
			}`,
			"/clients/0/client_id: Required",
		)
		// Authenticator
		test(`
			{
				"api_version": "v2.1",
				"master_key": "master_key",
				"asset": {
					"secret": "assetsecret"
				},
				"authentication": {
					"secret": "authnsessionsecret"
				},
				"identity": {
					"login_id": {
						"keys": [
							{
								"key": "email",
								"type": "email"
							},
							{
								"key": "phone",
								"type": "phone"
							},
							{
								"key": "username",
								"type": "username"
							}
						]
					}
				},
				"hook": {
					"secret": "hooksecret"
				},
				"authenticator": {
					"password": {
						"policy": {
							"min_length": -1,
							"minimum_guessable_level": 5,
							"history_size": -1,
							"history_days": -1,
							"expiry_days": -1
						}
					},
					"totp": {
						"maximum": 1000
					},
					"oob_otp": {
						"sms": {
							"maximum": 1000
						},
						"email": {
							"maximum": 1000
						}
					},
					"bearer_token": {
						"expire_in_days": 0
					},
					"recovery_code": {
						"count": 100,
						"list_enabled": 1
					}
				}
			}`,
			"/authenticator/bearer_token/expire_in_days: NumberRange map[gte:1]",
			"/authenticator/oob_otp/email/maximum: NumberRange map[lte:999]",
			"/authenticator/oob_otp/sms/maximum: NumberRange map[lte:999]",
			"/authenticator/password/policy/expiry_days: NumberRange map[gte:0]",
			"/authenticator/password/policy/history_days: NumberRange map[gte:0]",
			"/authenticator/password/policy/history_size: NumberRange map[gte:0]",
			"/authenticator/password/policy/min_length: NumberRange map[gte:0]",
			"/authenticator/password/policy/minimum_guessable_level: NumberRange map[lte:4]",
			"/authenticator/recovery_code/count: NumberRange map[lte:24]",
			"/authenticator/recovery_code/list_enabled: Type map[expected:boolean]",
			"/authenticator/totp/maximum: NumberRange map[lte:999]",
		)
		// WelcomeMessageConfiguration
		test(`
			{
				"api_version": "v2.1",
				"master_key": "master_key",
				"asset": {
					"secret": "assetsecret"
				},
				"authentication": {
					"secret": "authnsessionsecret"
				},
				"identity": {
					"login_id": {
						"keys": [
							{
								"key": "email",
								"type": "email"
							},
							{
								"key": "phone",
								"type": "phone"
							},
							{
								"key": "username",
								"type": "username"
							}
						]
					}
				},
				"hook": {
					"secret": "hooksecret"
				},
				"welcome_message": {
					"destination": "invalid"
				}
			}`,
			"/welcome_message/destination: Enum map[expected:[first all]]",
		)
		// OAuth
		test(`
			{
				"api_version": "v2.1",
				"master_key": "master_key",
				"asset": {
					"secret": "assetsecret"
				},
				"authentication": {
					"secret": "authnsessionsecret"
				},
				"hook": {
					"secret": "hooksecret"
				},
				"identity": {
					"login_id": {
						"keys": [
							{
								"key": "email",
								"type": "email"
							},
							{
								"key": "phone",
								"type": "phone"
							},
							{
								"key": "username",
								"type": "username"
							}
						]
					},
					"oauth": {
						"providers": [
							{ "type": "azureadv2" },
							{ "type": "google" },
							{ "type": "apple" }
						]
					}
				}
			}`,
			"/identity/oauth/providers/0/client_id: Required",
			"/identity/oauth/providers/0/client_secret: Required",
			"/identity/oauth/providers/0/tenant: Required",
			"/identity/oauth/providers/1/client_id: Required",
			"/identity/oauth/providers/1/client_secret: Required",
			"/identity/oauth/providers/2/client_id: Required",
			"/identity/oauth/providers/2/client_secret: Required",
			"/identity/oauth/providers/2/key_id: Required",
			"/identity/oauth/providers/2/team_id: Required",
			"/identity/oauth/state_jwt_secret: Required",
		)
		// UserVerificationConfiguration
		test(`
			{
				"api_version": "v2.1",
				"master_key": "master_key",
				"asset": {
					"secret": "assetsecret"
				},
				"authentication": {
					"secret": "authnsessionsecret"
				},
				"identity": {
					"login_id": {
						"keys": [
							{
								"key": "email",
								"type": "email"
							},
							{
								"key": "phone",
								"type": "phone"
							},
							{
								"key": "username",
								"type": "username"
							}
						]
					}
				},
				"hook": {
					"secret": "hooksecret"
				},
				"user_verification": {
					"criteria": "invalid",
					"login_id_keys": [
						{
							"key": "email",
							"code_format": "invalid"
						}
					]
				}
			}`,
			"/user_verification/criteria: Enum map[expected:[any all]]",
			"/user_verification/login_id_keys/0/code_format: Enum map[expected:[numeric complex]]",
		)
		// SMTP config
		test(`
			{
				"api_version": "v2.1",
				"master_key": "master_key",
				"asset": {
					"secret": "assetsecret"
				},
				"authentication": {
					"secret": "authnsessionsecret"
				},
				"identity": {
					"login_id": {
						"keys": [
							{
								"key": "email",
								"type": "email"
							},
							{
								"key": "phone",
								"type": "phone"
							},
							{
								"key": "username",
								"type": "username"
							}
						]
					}
				},
				"hook": {
					"secret": "hooksecret"
				},
				"smtp": {
					"mode": "invalid"
				}
			}`,
			"/smtp/mode: Enum map[expected:[normal ssl]]",
		)
		// Nexmo config
		test(`
			{
				"api_version": "v2.1",
				"master_key": "master_key",
				"asset": {
					"secret": "assetsecret"
				},
				"authentication": {
					"secret": "authnsessionsecret"
				},
				"identity": {
					"login_id": {
						"keys": [
							{
								"key": "email",
								"type": "email"
							},
							{
								"key": "phone",
								"type": "phone"
							},
							{
								"key": "username",
								"type": "username"
							}
						]
					}
				},
				"hook": {
					"secret": "hooksecret"
				},
				"nexmo": {
					"api_secret": 1
				}
			}`,
			"/nexmo/api_secret: Type map[expected:string]",
		)
		// Country calling code - bad
		test(`
			{
				"api_version": "v2.1",
				"master_key": "master_key",
				"asset": {
					"secret": "assetsecret"
				},
				"authentication": {
					"secret": "authnsessionsecret"
				},
				"identity": {
					"login_id": {
						"keys": [
							{
								"key": "email",
								"type": "email"
							},
							{
								"key": "phone",
								"type": "phone"
							},
							{
								"key": "username",
								"type": "username"
							}
						]
					}
				},
				"hook": {
					"secret": "hooksecret"
				},
				"auth_ui": {
					"country_calling_code": {
						"default": "a"
					}
				}
			}`,
			"/auth_ui/country_calling_code/default: StringFormat map[pattern:^\\d+$]",
		)
		// Country calling code - good
		test(`
			{
				"api_version": "v2.1",
				"master_key": "master_key",
				"asset": {
					"secret": "assetsecret"
				},
				"authentication": {
					"secret": "authnsessionsecret"
				},
				"identity": {
					"login_id": {
						"keys": [
							{
								"key": "email",
								"type": "email"
							},
							{
								"key": "phone",
								"type": "phone"
							},
							{
								"key": "username",
								"type": "username"
							}
						]
					}
				},
				"hook": {
					"secret": "hooksecret"
				},
				"auth_ui": {
					"country_calling_code": {
						"default": "852"
					}
				}
			}`,
		)
	})
}
