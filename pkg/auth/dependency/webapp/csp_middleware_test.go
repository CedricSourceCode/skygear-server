package webapp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/skygeario/skygear-server/pkg/core/config"
)

func TestCSPMiddleware(t *testing.T) {
	Convey("CSPMiddleware", t, func() {
		dummy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
		middleware := &CSPMiddleware{}
		h := middleware.Handle(dummy)

		Convey("no clients", func() {
			w := httptest.NewRecorder()
			r, _ := http.NewRequest("GET", "/", nil)
			h.ServeHTTP(w, r)

			So(w.Result().Header.Get("Content-Security-Policy"), ShouldEqual, "frame-ancestors 'self';")
		})

		Convey("one client", func() {
			middleware.Clients = []config.OAuthClientConfiguration{
				config.OAuthClientConfiguration{
					"redirect_uris": []interface{}{
						"https://example.com/path?q=1",
					},
				},
			}
			w := httptest.NewRecorder()
			r, _ := http.NewRequest("GET", "/", nil)
			h.ServeHTTP(w, r)

			So(w.Result().Header.Get("Content-Security-Policy"), ShouldEqual, "frame-ancestors https://example.com 'self';")
		})

		Convey("more than one clients", func() {
			middleware.Clients = []config.OAuthClientConfiguration{
				config.OAuthClientConfiguration{
					"redirect_uris": []interface{}{
						"https://example.com/path?q=1",
					},
				},
				config.OAuthClientConfiguration{
					"redirect_uris": []interface{}{
						"https://app.com/path?q=1",
					},
				},
			}
			w := httptest.NewRecorder()
			r, _ := http.NewRequest("GET", "/", nil)
			h.ServeHTTP(w, r)

			So(w.Result().Header.Get("Content-Security-Policy"), ShouldEqual, "frame-ancestors https://example.com https://app.com 'self';")
		})

		Convey("include https redirect URIs", func() {
			middleware.Clients = []config.OAuthClientConfiguration{
				config.OAuthClientConfiguration{
					"redirect_uris": []interface{}{
						"https://example.com/path?q=1",
						"http://example.com/path?q=1",
						"com.example://host/path",
					},
				},
			}
			w := httptest.NewRecorder()
			r, _ := http.NewRequest("GET", "/", nil)
			h.ServeHTTP(w, r)

			So(w.Result().Header.Get("Content-Security-Policy"), ShouldEqual, "frame-ancestors https://example.com 'self';")
		})

		Convey("include http redirect URIs if host is localhost", func() {
			middleware.Clients = []config.OAuthClientConfiguration{
				config.OAuthClientConfiguration{
					"redirect_uris": []interface{}{
						"http://127.0.0.1/path?q=1",
						"http://127.0.0.1:8080/path?q=1",
						"http://[::1]/path?q=1",
						"http://[::1]:8080/path?q=1",
						"http://localhost/path?q=1",
						"http://localhost:8080/path?q=1",
						"http://skygear.localhost/path?q=1",
						"http://skygear.localhost:8080/path?q=1",

						"http://example.com/path?q=1",
						"http://192.168.1.1/path?q=1",
						"http://skygearlocalhost/path?q=1",
					},
				},
			}
			w := httptest.NewRecorder()
			r, _ := http.NewRequest("GET", "/", nil)
			h.ServeHTTP(w, r)

			So(w.Result().Header.Get("Content-Security-Policy"), ShouldEqual, "frame-ancestors http://127.0.0.1 http://127.0.0.1:8080 http://[::1] http://[::1]:8080 http://localhost http://localhost:8080 http://skygear.localhost http://skygear.localhost:8080 'self';")
		})
	})
}
