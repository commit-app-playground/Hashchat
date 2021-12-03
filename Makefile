APP_NAME = hashtag

# Creates (or updates) secrets object on the k8s cluster server
upsert-secrets:
	kubectl apply -n hashchat -f secrets/secrets.yml

init:
	go get github.com/go-swagger/go-swagger/cmd/swagger

dep:
	go mod tidy

build:
	go build -o $(APP_NAME) cmd/server/main.go

generate:
	swagger generate server -A $(APP_NAME) -f swagger/swagger.yml --principal=models.Principal --exclude-main

run-server: build
	./$(APP_NAME)