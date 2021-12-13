// This file is safe to edit. Once it exists it will not be overwritten

package restapi

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

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

type WebSocketMessage struct {
	ChatMessage *ChatMessage
	MoveChannel *MoveChannel
}

type ChatMessage struct {
	Username          string    `json:"username"`
	Text              string    `json:"text"`
	Time              time.Time `json:"time"`
	HashId            string    `json:"hashId"`
	ActiveConnections int64     `json:"activeConnections"`
	ActiveUsers       []string  `json:"activeUsers"`
}

type MoveChannel struct {
	Username string `json:"username"`
	HashId   string `json:"hashId"`
}

type loggingWrapper struct {
	defaultHandler http.Handler
	loggingHandler http.Handler
}

var (
	rdb *redis.Client
)

var clients = make(map[*websocket.Conn]string)
var channelConnections = make(map[string]int64)
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

	return middleware.ResponderFunc(func(rw http.ResponseWriter, p runtime.Producer) {
		ws, err := upgrader.Upgrade(rw, params.HTTPRequest, nil)
		if err != nil {
			log.Fatal(err)
		}
		// ensure connection close when function returns
		defer ws.Close()
		clients[ws] = ""

		for {
			var wsmsg WebSocketMessage
			// Read in a new message as JSON and map it to a WebSocket object

			err := ws.ReadJSON(&wsmsg)
			if err != nil {
				hashtagId := clients[ws]

				delete(clients, ws)
				decrementChannelUsers(hashtagId)
				break
			}

			// send new message to the channel
			if wsmsg.ChatMessage != nil {
				broadcaster <- *wsmsg.ChatMessage

				// move channels, send any existing messages
			} else if wsmsg.MoveChannel != nil {
				oldTag := clients[ws]
				if oldTag != "" {
					decrementChannelUsers(oldTag)
				}
				clients[ws] = wsmsg.MoveChannel.HashId

				multiHashtagChannel := strings.Split(wsmsg.MoveChannel.HashId, ",")

				// either multihastag channel or single hastag channel
				if len(multiHashtagChannel) > 1 {
					for _, c := range multiHashtagChannel {
						channelConnections[c] += 1
					}
					hashtagMessages := grabMultiChannelChatMessages(multiHashtagChannel)

					for _, chatMessage := range hashtagMessages {
						chatMessage.ActiveConnections = channelConnections[chatMessage.HashId]
						messageClient(ws, chatMessage)
					}
				} else {
					channelConnections[wsmsg.MoveChannel.HashId] += 1
					if rdb.Exists(wsmsg.MoveChannel.HashId).Val() != 0 {
						sendPreviousMessages(ws, wsmsg.MoveChannel.HashId)
					}
				}
			}
		}
	})
}

func incrementChannelUsers(hashtagId string) {
	multiHashtagChannel := strings.Split(hashtagId, ",")
	if len(multiHashtagChannel) > 1 {
		for _, c := range multiHashtagChannel {
			channelConnections[c] += 1
		}
	} else {
		channelConnections[hashtagId] += 1
	}
}

func decrementChannelUsers(hashtagId string) {
	multiHashtagChannel := strings.Split(hashtagId, ",")
	if len(multiHashtagChannel) > 1 {
		for _, c := range multiHashtagChannel {
			channelConnections[c] -= 1
		}
	} else {
		channelConnections[hashtagId] -= 1
	}
}

func grabChatMessages(hashtagId string) []string {

	chatMessages, err := rdb.LRange(hashtagId, 0, -1).Result()
	if err != nil {
		panic(err)
	}
	return chatMessages
}

func grabMultiChannelChatMessages(hashtags []string) []ChatMessage {
	var allStringMessages []string
	var msgs []ChatMessage

	for _, key := range hashtags {
		allStringMessages = append(allStringMessages, grabChatMessages(key)...)
	}

	for _, chatMessage := range allStringMessages {
		var msg ChatMessage
		log.Println(chatMessage)
		json.Unmarshal([]byte(chatMessage), &msg)
		msgs = append(msgs, msg)
	}

	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].Time.Before(msgs[j].Time)
	})

	return msgs
}

func sendPreviousMessages(ws *websocket.Conn, hashtagId string) {
	chatMessages := grabChatMessages(hashtagId)
	// send previous messages
	for _, chatMessage := range chatMessages {
		var msg ChatMessage
		json.Unmarshal([]byte(chatMessage), &msg)
		msg.ActiveConnections = channelConnections[hashtagId]
		messageClient(ws, msg)
	}
}

// If a message is sent while a client is closing, ignore the error
func unsafeError(err error) bool {
	return !websocket.IsCloseError(err, websocket.CloseGoingAway) && err != io.EOF
}

func handleMessages() {
	for {
		// grab msg from channel
		msg := <-broadcaster

		// update redis & update users
		storeInRedis(msg)
		messageClients(msg)

	}
}

func messageClients(msg ChatMessage) {
	// send to every client currently connected
	for client, hashId := range clients {
		multiHashtagChannel := strings.Split(hashId, ",")
		log.Println(multiHashtagChannel)

		for _, c := range multiHashtagChannel {
			if c == msg.HashId {
				messageClient(client, msg)
			}
		}
	}
}

func messageClient(client *websocket.Conn, msg ChatMessage) {
	err := client.WriteJSON(msg)
	if err != nil && unsafeError(err) {
		client.Close()
		delete(clients, client)
		decrementChannelUsers(msg.HashId)
	}
}

func storeInRedis(msg ChatMessage) {
	json, err := json.Marshal(msg)
	if err != nil {
		panic(err)
	}

	if err := rdb.RPush(msg.HashId, json).Err(); err != nil {
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
	// local only
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
