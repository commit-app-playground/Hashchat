package controllers

import (
	"github.com/commit-app-playground/Hashchat/cmd/server/controllers/hashtag"
	"github.com/commit-app-playground/Hashchat/cmd/server/controllers/user"
	"github.com/go-redis/redis"
)

type AllControllers struct {
	Hashtag *hashtag.HashtagController
	User    *user.UserController
}

func NewAllControllers(redis *redis.Client) *AllControllers {
	return &AllControllers{
		Hashtag: hashtag.NewJobController(redis),
		User:    user.NewUserController(redis),
	}
}
