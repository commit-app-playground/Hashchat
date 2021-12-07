package controllers

import (
	"github.com/commit-app-playground/Hashchat/cmd/server/controllers/hashtag"
	"github.com/commit-app-playground/Hashchat/cmd/server/controllers/user"
)

type AllControllers struct {
	Hashtag *hashtag.HashtagController
	User    *user.UserController
}

func NewAllControllers() *AllControllers {
	return &AllControllers{
		Hashtag: hashtag.NewJobController(),
		User:    user.NewUserController(),
	}
}
