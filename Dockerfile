
FROM golang:1.13-stretch AS build

## setup project path
WORKDIR /cache/hashchat

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

# Allow glibc on alpine
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

COPY --from=build /cache/hashchat /app/
## execute binary as container entrypoint
EXPOSE 80

ENTRYPOINT /app/hashchat