package controllers

import "github.com/commit-app-playground/Hashchat/cmd/server/controllers/hashtag"

type AllControllers struct {
	Hashtag *hashtag.HashtagController
}

func NewAllControllers() *AllControllers {
	return &AllControllers{
		Hashtag: hashtag.NewJobController(),
	}
}
