package main

import (
	"log"

	"github.com/commit-app-playground/Hashchat/restapi"
	"github.com/commit-app-playground/Hashchat/restapi/operations"
	"github.com/go-openapi/loads"
)

func main() {
	swaggerSpec, err := loads.Embedded(restapi.SwaggerJSON, restapi.FlatSwaggerJSON)
	if err != nil {
		log.Fatalln(err)
	}

	api := operations.NewHashchatAPI(swaggerSpec)

	server := restapi.NewServer(api)
	defer server.Shutdown()

	server.Host = "0.0.0.0"
	server.Port = 80
	server.EnabledListeners = []string{"http", "ws"}

	server.ConfigureAPI()

	if err := server.Serve(); err != nil {
		log.Fatalln(err)
	}
}
