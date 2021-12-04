// This file is safe to edit. Once it exists it will not be overwritten

package restapi

import (
	"crypto/tls"
	"net/http"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"

	"github.com/commit-app-playground/Hashchat/cmd/server/controllers"
	"github.com/commit-app-playground/Hashchat/restapi/operations"
	"github.com/commit-app-playground/Hashchat/restapi/operations/hashtags"
	"github.com/commit-app-playground/Hashchat/restapi/operations/health"
)

//go:generate swagger generate server --target ../../Hashchat --name Hashchat --spec ../swagger/swagger.yml --principal models.Principal --exclude-main

func configureFlags(api *operations.HashchatAPI) {
	// api.CommandLineOptionsGroups = []swag.CommandLineOptionsGroup{ ... }
}

func configureAPI(api *operations.HashchatAPI) http.Handler {
	// configure the api here
	api.ServeError = errors.ServeError

	// Set your custom logger if needed. Default one is log.Printf
	// Expected interface func(string, ...interface{})
	//
	// Example:
	// api.Logger = log.Printf

	api.UseSwaggerUI()
	// To continue using redoc as your UI, uncomment the following line
	// api.UseRedoc()

	api.JSONConsumer = runtime.JSONConsumer()

	api.JSONProducer = runtime.JSONProducer()

	c := controllers.NewAllControllers()

	api.HashtagsGetHashtagMessagesHandler = hashtags.GetHashtagMessagesHandlerFunc(c.Hashtag.GetHashtagMessages)

	// if api.HashtagsInsertHashtagMessageHandler == nil {
	// 	api.HashtagsInsertHashtagMessageHandler = hashtags.InsertHashtagMessageHandlerFunc(func(params hashtags.InsertHashtagMessageParams) middleware.Responder {
	// 		return middleware.NotImplemented("operation hashtags.InsertHashtagMessage has not yet been implemented")
	// 	})
	// }

	//Health
	api.HealthGetLivenessHandler = health.GetLivenessHandlerFunc(func(params health.GetLivenessParams) middleware.Responder {
		return health.NewGetLivenessOK().WithPayload("OK")
	})
	api.HealthGetReadinessHandler = health.GetReadinessHandlerFunc(func(params health.GetReadinessParams) middleware.Responder {
		return health.NewGetReadinessOK().WithPayload("OK")
	})

	api.PreServerShutdown = func() {}

	api.ServerShutdown = func() {}

	return setupGlobalMiddleware(api.Serve(setupMiddlewares))
}

// The TLS configuration before HTTPS server starts.
func configureTLS(tlsConfig *tls.Config) {
	// Make all necessary changes to the TLS configuration here.
}

// As soon as server is initialized but not run yet, this function will be called.
// If you need to modify a config, store server instance to stop it individually later, this is the place.
// This function can be called multiple times, depending on the number of serving schemes.
// scheme value will be set accordingly: "http", "https" or "unix".
func configureServer(s *http.Server, scheme, addr string) {
}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation.
func setupMiddlewares(handler http.Handler) http.Handler {
	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics.
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	return handler
}
