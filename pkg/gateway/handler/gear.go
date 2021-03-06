package handler

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/skygeario/skygear-server/pkg/core/errors"
	coreHttp "github.com/skygeario/skygear-server/pkg/core/http"
	"github.com/skygeario/skygear-server/pkg/core/inject"
	"github.com/skygeario/skygear-server/pkg/core/logging"
	"github.com/skygeario/skygear-server/pkg/core/sentry"
	"github.com/skygeario/skygear-server/pkg/gateway"
	"github.com/skygeario/skygear-server/pkg/gateway/config"
	"github.com/skygeario/skygear-server/pkg/gateway/model"
)

type GearHandlerFactory struct {
	Dependency gateway.DependencyMap
}

func (f *GearHandlerFactory) NewHandler(request *http.Request) http.Handler {
	h := &GearHandler{}
	inject.DefaultRequestInject(h, f.Dependency, request)
	return h
}

type GearHandler struct {
	GatewayConfiguration config.Configuration `dependency:"GatewayConfiguration"`
}

// NewGearHandler takes an incoming request and sends it to coresponding
// gear server
func (h *GearHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	loggerFactory := logging.NewFactory(
		logging.NewDefaultLogHook(nil),
		sentry.NewLogHookFromContext(r.Context()),
	)
	logger := loggerFactory.NewLogger("gear-handler")

	ctx := model.GatewayContextFromContext(r.Context())
	app := ctx.App
	gear := ctx.Gear

	// check if app support given gear
	gearVersion := app.GetGearVersion(gear)
	if !app.CanAccessGear(gear) {
		http.Error(rw, fmt.Sprintf("%s is not support in current app plan", gear), http.StatusForbidden)
		return
	}

	gearURL, err := h.GatewayConfiguration.GetGearURL(gear, gearVersion)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return
	}

	if gearURL == "" {
		logger.Error(fmt.Sprintf("%s gear %s environment is not supported", gear, gearVersion))
		http.Error(rw, "Internal server error", http.StatusInternalServerError)
		return
	}

	director := func(req *http.Request) {
		path := req.URL.Path
		query := req.URL.RawQuery
		fragment := req.URL.Fragment
		coreHttp.SetForwardedHeaders(req)

		var err error
		u, err := url.Parse(gearURL)
		if err != nil {
			panic(errors.Newf("failed to parse gear endpoint:%w", err))
		}
		req.URL = u
		req.URL.Path = path
		req.URL.RawQuery = query
		req.URL.Fragment = fragment
	}
	modifyResponse := func(resp *http.Response) error {
		coreHttp.FixupCORSHeaders(rw, resp)
		return nil
	}

	proxy := &httputil.ReverseProxy{
		Director:       director,
		ModifyResponse: modifyResponse,
		ErrorHandler:   reverseProxyErrorHandler,
	}
	proxy.ServeHTTP(rw, r)
}
