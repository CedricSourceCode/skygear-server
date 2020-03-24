// Code generated by Wire. DO NOT EDIT.

//go:generate wire
//+build !wireinject

package auth

import (
	"github.com/gorilla/mux"
	auth2 "github.com/skygeario/skygear-server/pkg/auth/dependency/auth"
	redis2 "github.com/skygeario/skygear-server/pkg/auth/dependency/auth/redis"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/session"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/session/redis"
	"github.com/skygeario/skygear-server/pkg/auth/dependency/webapp"
	"github.com/skygeario/skygear-server/pkg/core/auth"
	"github.com/skygeario/skygear-server/pkg/core/auth/authinfo/pq"
	"github.com/skygeario/skygear-server/pkg/core/db"
	"github.com/skygeario/skygear-server/pkg/core/logging"
	"github.com/skygeario/skygear-server/pkg/core/time"
	"net/http"
)

// Injectors from wire.go:

func NewAccessKeyMiddleware(r *http.Request, m DependencyMap) mux.MiddlewareFunc {
	context := ProvideContext(r)
	tenantConfiguration := ProvideTenantConfig(context)
	accessKeyMiddleware := auth.ProvideAccessKeyMiddleware(tenantConfiguration)
	middlewareFunc := provideMiddleware(accessKeyMiddleware)
	return middlewareFunc
}

func NewSessionMiddleware(r *http.Request, m DependencyMap) mux.MiddlewareFunc {
	insecureCookieConfig := ProvideSessionInsecureCookieConfig(m)
	context := ProvideContext(r)
	tenantConfiguration := ProvideTenantConfig(context)
	cookieConfiguration := session.ProvideSessionCookieConfiguration(r, insecureCookieConfig, tenantConfiguration)
	provider := time.NewProvider()
	requestID := ProvideLoggingRequestID(r)
	factory := logging.ProvideLoggerFactory(context, requestID, tenantConfiguration)
	store := redis.ProvideStore(context, tenantConfiguration, provider, factory)
	eventStore := redis2.ProvideEventStore(context, tenantConfiguration)
	accessEventProvider := auth2.AccessEventProvider{
		Store: eventStore,
	}
	sessionProvider := session.ProvideSessionProvider(r, store, accessEventProvider, tenantConfiguration)
	resolver := session.ProvideSessionResolver(cookieConfiguration, sessionProvider)
	sqlBuilderFactory := db.ProvideSQLBuilderFactory(tenantConfiguration)
	sqlExecutor := db.ProvideSQLExecutor(context, tenantConfiguration)
	authinfoStore := pq.ProvideStore(sqlBuilderFactory, sqlExecutor)
	txContext := db.ProvideTxContext(context, tenantConfiguration)
	middleware := &auth2.Middleware{
		IDPSessionResolver: resolver,
		AccessEvents:       accessEventProvider,
		AuthInfoStore:      authinfoStore,
		TxContext:          txContext,
	}
	middlewareFunc := provideMiddleware(middleware)
	return middlewareFunc
}

func NewCSPMiddleware(r *http.Request, m DependencyMap) mux.MiddlewareFunc {
	context := ProvideContext(r)
	tenantConfiguration := ProvideTenantConfig(context)
	middlewareFunc := webapp.ProvideCSPMiddleware(tenantConfiguration)
	return middlewareFunc
}

// wire.go:

type middlewareInstance interface {
	Handle(next http.Handler) http.Handler
}

func provideMiddleware(m middlewareInstance) mux.MiddlewareFunc {
	return m.Handle
}
