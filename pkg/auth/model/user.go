// Copyright 2015-present Oursky Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"time"

	"github.com/skygeario/skygear-server/pkg/auth/dependency/userprofile"
)

// User is the unify way of returning a AuthInfo with LoginID to SDK
type User struct {
	ID               string           `json:"id,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	LastLoginAt      *time.Time       `json:"last_login_at,omitempty"`
	Verified         bool             `json:"is_verified"`
	ManuallyVerified bool             `json:"is_manually_verified"`
	Disabled         bool             `json:"is_disabled"`
	IsAnonymous      bool             `json:"is_anonymous"`
	VerifyInfo       map[string]bool  `json:"verify_info"`
	Metadata         userprofile.Data `json:"metadata"`
}

// @JSONSchema
const UserSchema = `
{
	"$id": "#User",
	"type": "object",
	"properties": {
		"id": { "type": "string" },
		"created_at": { "type": "string" },
		"last_login_at": { "type": "string" },
		"is_verified": { "type": "boolean" },
		"is_manually_verified": { "type": "boolean" },
		"is_disabled": { "type": "boolean" },
		"is_anonymous": { "type": "boolean" },
		"verify_info": { "type": "object" },
		"metadata": { "type": "object" }
	}
}
`
