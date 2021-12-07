package user

import (
	"github.com/commit-app-playground/Hashchat/models"
	"github.com/commit-app-playground/Hashchat/restapi/operations/user"
	"github.com/go-openapi/runtime/middleware"
)

type UserController struct {
}

func NewUserController() *UserController {
	return &UserController{}
}

func (u *UserController) GetUserHashtagChannels(params user.GetUserHashtagChannelsParams) middleware.Responder {
	payload := models.UserHashtagChannels{}

	return user.NewGetUserHashtagChannelsOK().WithPayload(payload)

}
