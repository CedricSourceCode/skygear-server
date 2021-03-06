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

package middleware

import (
	"net/http"
	"strings"

	"github.com/iawaknahc/originmatcher"
	"github.com/skygeario/skygear-server/pkg/core/config"
	coreHttp "github.com/skygeario/skygear-server/pkg/core/http"
)

type CORSMiddleware struct {
}

func (cors CORSMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tConfig := config.GetTenantConfig(r.Context())
		matcher, err := originmatcher.Parse(tConfig.AppConfig.CORS.Origin)

		w.Header().Add("Vary", "Origin")

		origin := r.Header.Get("Origin")
		if origin != "" && err == nil && matcher.MatchOrigin(origin) {
			corsMethod := r.Header.Get("Access-Control-Request-Method")
			corsHeaders := r.Header.Get("Access-Control-Request-Headers")

			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "900") // 15 mins
			w.Header().Set("Access-Control-Expose-Headers", strings.Join([]string{coreHttp.HeaderTryRefreshToken}, ", "))

			if corsMethod != "" {
				w.Header().Set("Access-Control-Allow-Methods", corsMethod)
			}

			if corsHeaders != "" {
				w.Header().Set("Access-Control-Allow-Headers", corsHeaders)
			}
		}

		requestMethod := r.Method
		if requestMethod == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte{})
		} else {
			next.ServeHTTP(w, r)
		}
	})
}
