
FROM golang:1.13-stretch AS build

## setup project path
WORKDIR /go/src/github.com/commit-app-playground/Hashchat

COPY go.mod .
COPY go.sum .
COPY Makefile ./

## fetch dependencies
RUN go mod tidy && go mod download

## copy project files
COPY . .

## generate code
RUN go get github.com/go-swagger/go-swagger/cmd/swagger@v0.26.1
RUN make generate

## generate binary
RUN make build

## copy binary to an alpine image
FROM alpine
COPY --from=build /go/src/github.com/commit-app-playground/Hashchat /app/
## execute binary as container entrypoint
EXPOSE 80

ENTRYPOINT /app/Hashchat