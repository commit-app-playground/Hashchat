package hashtag

import (
	"github.com/commit-app-playground/Hashchat/models"
	"github.com/commit-app-playground/Hashchat/restapi/operations/hashtags"
	"github.com/go-openapi/runtime/middleware"
)

type HashtagController struct {
}

func NewJobController() *HashtagController {
	return &HashtagController{}
}

func (c *HashtagController) GetHashtagMessages(params hashtags.GetHashtagMessagesParams) middleware.Responder {
	payload := models.HashtagMessagesResponse{HashtagID: "12345"}

	return hashtags.NewGetHashtagMessagesOK().WithPayload(&payload)

}
