// This file is safe to edit. Once it exists it will not be overwritten

package restapi

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"

	"github.com/commit-app-playground/Hashchat/cmd/server/controllers"
	"github.com/commit-app-playground/Hashchat/restapi/operations"
	"github.com/commit-app-playground/Hashchat/restapi/operations/hashtags"
	"github.com/commit-app-playground/Hashchat/restapi/operations/health"
	"github.com/commit-app-playground/Hashchat/restapi/operations/user"
	ws "github.com/commit-app-playground/Hashchat/restapi/operations/websocket"
)

type ChatMessage struct {
	Username string `json:"username"`
	Text     string `json:"text"`
	Time     string `json:"time"`
	HashId   string `json:"hashId"`
}

type loggingWrapper struct {
	defaultHandler http.Handler
	loggingHandler http.Handler
}

var (
	rdb *redis.Client
)

var clients = make(map[*websocket.Conn]bool)
var broadcaster = make(chan ChatMessage)
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

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

	// setup redis client and log ping for healthcheck
	rdb = redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
		Password: "",
		DB:       0,
	})
	pong, err := rdb.Ping().Result()
	fmt.Println(pong, err)

	c := controllers.NewAllControllers(rdb)

	// handle incoming websocket connections
	api.WebsocketConnectWebsocketHandler = ws.ConnectWebsocketHandlerFunc(handleConnections)
	go handleMessages()

	api.HashtagsGetHashtagMessagesHandler = hashtags.GetHashtagMessagesHandlerFunc(c.Hashtag.GetHashtagMessages)

	api.UserGetUserHashtagChannelsHandler = user.GetUserHashtagChannelsHandlerFunc(c.User.GetUserHashtagChannels)
	api.UserPostUserHashtagHandler = user.PostUserHashtagHandlerFunc(c.User.PostUserHashtag)
	api.UserInsertHashtagsForUserHandler = user.InsertHashtagsForUserHandlerFunc(c.User.InsertHashtagsForUser)

	if api.HashtagsInsertHashtagMessageHandler == nil {
		api.HashtagsInsertHashtagMessageHandler = hashtags.InsertHashtagMessageHandlerFunc(func(params hashtags.InsertHashtagMessageParams) middleware.Responder {
			return middleware.NotImplemented("operation hashtags.InsertHashtagMessage has not yet been implemented")
		})
	}

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

// handle incoming client connections
func handleConnections(params ws.ConnectWebsocketParams) middleware.Responder {
	log.Println("handleConnections")

	return middleware.ResponderFunc(func(rw http.ResponseWriter, p runtime.Producer) {
		log.Println("yurt")
		ws, err := upgrader.Upgrade(rw, params.HTTPRequest, nil)
		if err != nil {
			log.Fatal(err)
		}
		// ensure connection close when function returns
		defer ws.Close()
		clients[ws] = true

		// if it's zero, no messages were ever sent/saved
		if rdb.Exists("chat_messages").Val() != 0 {
			sendPreviousMessages(ws)
		}

		for {
			log.Println("for handleConnections")

			var msg ChatMessage
			// Read in a new message as JSON and map it to a Message object
			log.Println(msg)

			err := ws.ReadJSON(&msg)
			if err != nil {
				log.Println("for handleConnections err")
				log.Println(err)

				delete(clients, ws)
				break
			}
			// send new message to the channel
			broadcaster <- msg
		}
	})
}

func sendPreviousMessages(ws *websocket.Conn) {
	log.Println("sendPreviousMessages")

	chatMessages, err := rdb.LRange("chat_messages", 0, -1).Result()
	if err != nil {
		panic(err)
	}
	log.Println(chatMessages)

	// send previous messages
	for _, chatMessage := range chatMessages {
		var msg ChatMessage
		json.Unmarshal([]byte(chatMessage), &msg)
		messageClient(ws, msg)
	}
}

// If a message is sent while a client is closing, ignore the error
func unsafeError(err error) bool {
	log.Println("unsafeError")

	return !websocket.IsCloseError(err, websocket.CloseGoingAway) && err != io.EOF
}

func handleMessages() {
	log.Println("handling messages")
	for {
		// grab msg from 	channel
		msg := <-broadcaster

		// update redis & update users
		storeInRedis(msg)
		messageClients(msg)

	}
}

func messageClients(msg ChatMessage) {
	log.Println("messageClients")

	log.Println(msg)

	// send to every client currently connected
	for client := range clients {
		messageClient(client, msg)
	}
}

func messageClient(client *websocket.Conn, msg ChatMessage) {
	log.Println("messageClient")

	err := client.WriteJSON(msg)
	if err != nil && unsafeError(err) {
		log.Printf("error: %v", err)
		client.Close()
		delete(clients, client)
	}
}

func storeInRedis(msg ChatMessage) {
	log.Println("storeInRedis")

	log.Println(msg)
	json, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}

	if err := rdb.RPush("chat_messages", json).Err(); err != nil {
		panic(err)
	}
}

// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
// The middleware executes after routing but before authentication, binding and validation.
func setupMiddlewares(handler http.Handler) http.Handler {
	return handler
}

// The middleware configuration happens before anything, this middleware also applies to serving the swagger.json document.
// So this is a good place to plug in a panic handling middleware, logging and metrics.
func setupGlobalMiddleware(handler http.Handler) http.Handler {
	// handler = HandleCORS(handler)

	return addLogging(handler)
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

// func HandleCORS(handler http.Handler) http.Handler {
// 	corsHandler := cors.New(cors.Options{
// 		Debug:            false,
// 		AllowedOrigins:   []string{"http://localhost:8080", "http://localhost:8081"},
// 		AllowedHeaders:   []string{"*"},
// 		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodPut},
// 		AllowCredentials: true,
// 		MaxAge:           1000,
// 	})
// 	return corsHandler.Handler(handler)
// }

func addLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("received request:", r.Method, r.URL, r.Body)
		next.ServeHTTP(w, r)
	})
}
