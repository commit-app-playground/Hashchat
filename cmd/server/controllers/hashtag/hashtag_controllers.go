package hashtag

import (
	"github.com/commit-app-playground/Hashchat/models"
	"github.com/commit-app-playground/Hashchat/restapi/operations/hashtags"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-redis/redis"
)

type HashtagController struct {
	redis *redis.Client
}

func NewJobController(r *redis.Client) *HashtagController {
	return &HashtagController{
		redis: r,
	}
}

func (c *HashtagController) GetHashtagMessages(params hashtags.GetHashtagMessagesParams) middleware.Responder {
	payload := models.HashtagMessagesResponse{HashtagID: "12345"}

	return hashtags.NewGetHashtagMessagesOK().WithPayload(&payload)

}
