package user

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/commit-app-playground/Hashchat/models"
	"github.com/commit-app-playground/Hashchat/restapi/operations/user"
	"github.com/go-openapi/runtime/middleware"
	"github.com/go-redis/redis"
	"github.com/google/uuid"
)

const (
	keyUsers                  = "users"
	keyUserStatus             = "userStatus"
	keyUserChannels           = "userChannels"
	keyUserAccessKey          = "userAccessKey"
	keyUsersUUIDListIndex     = "usersUUIDListIndex"
	keyUsersUsernameListIndex = "usersUsernameListIndex"
)

type UserController struct {
	redis *redis.Client
}

func NewUserController(r *redis.Client) *UserController {
	return &UserController{
		redis: r,
	}
}

type User struct {
	UUID     string `json:"uuid" binding:"required"`
	Username string `json:"username" binding:"required"`
}

type UserChannel struct {
	Username  string `json:"username" binding:"required"`
	HashtagId int    `json:"hashtagId" binding:"required"`
}

var ctx = context.Background()

func (u *UserController) GetUserHashtagChannels(params user.GetUserHashtagChannelsParams) middleware.Responder {
	log.Println("GetUserHashtagChannels")

	payload := models.UserHashtagChannels{}

	fu, err := u.getUserFromListByUsername(params.Username)
	if err != nil {
		log.Println("User not PRESENT")

		log.Println(err)

		newUser := &User{
			UUID:     uuid.NewString(),
			Username: params.Username,
		}

		if err := u.addUser(newUser); err != nil {
			log.Println("Unable to add user, failed with %s", err.Error())
			return user.NewGetUserHashtagChannelsDefault(401)
		}
	} else {
		log.Println((fu))
	}

	sFindKey := fmt.Sprintf("%s-channels:Ids", params.Username)
	channels := u.redis.SMembers(sFindKey).Val()

	for _, channel := range channels {
		payload = append(payload, channel)

	}

	return user.NewGetUserHashtagChannelsOK().WithPayload(payload)

}

func (u *UserController) PostUserChange(params user.PostUserChangeParams) middleware.Responder {
	log.Println("PostUserChange")
	payload := models.UserHashtagChannels{}
	return user.NewPostUserChangeOK().WithPayload(payload)

}

func (u *UserController) InsertHashtagsForUser(params user.InsertHashtagsForUserParams) middleware.Responder {
	log.Println("InsertHashtagsForUser")
	payload := &models.HashtagResponse{}

	userKey := fmt.Sprintf("%s-channels:%s", params.Username, params.UserHashtag.HashtagID)
	_, err := u.redis.HMSet(userKey, map[string]interface{}{
		"hashtagId": params.UserHashtag.HashtagID,
	}).Result()
	if err != nil {
		fmt.Println(err.Error())
		return user.NewInsertHashtagsForUserDefault(401)
	}

	sAddKey := fmt.Sprintf("%s-channels:Ids", params.Username)
	_, err = u.redis.SAdd(sAddKey, params.UserHashtag.HashtagID).Result()
	if err != nil {
		fmt.Println(err.Error())
		return user.NewInsertHashtagsForUserDefault(401)
	}

	return user.NewInsertHashtagsForUserOK().WithPayload(payload)

}

func (u *UserController) getUserFromListByUsername(username string) (*User, error) {
	log.Println("getUserFromListByUsername", username)

	userIndex, err := u.getUserIndexByUsername(username)
	if err != nil {
		return nil, err
	}

	user, err := u.getUserFromList(userIndex)
	if err != nil {
		return nil, err
	}

	return user, nil

}

func (u *UserController) getUserIndexByUsername(username string) (int64, error) {
	log.Println("getUserIndexByUsername", username)

	key := getKeyUsersUsernameListIndex(username)
	value, err := u.redis.Get(key).Result()
	if err != nil {
		return 0, err
	}
	index, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return index, nil
}

func (u *UserController) getUserFromList(userIndex int64) (*User, error) {
	log.Println("getUserFromList")

	key := getKeyUsers()

	value, err := u.redis.LIndex(key, userIndex).Result()
	if err != nil {
		return nil, fmt.Errorf("getUserFromList[%d]: %w", userIndex, err)
	}

	user := &User{}

	dec := json.NewDecoder(strings.NewReader(value))
	err = dec.Decode(user)
	if err != nil {
		return nil, fmt.Errorf("getUserFromList[%d]: %w", userIndex, err)
	}

	return user, nil
}

func (u *UserController) addUser(user *User) error {
	log.Println("addUser")

	buff := bytes.NewBufferString("")
	enc := json.NewEncoder(buff)
	err := enc.Encode(user)
	if err != nil {
		return err
	}

	key := getKeyUsers()

	elements, err := u.redis.RPush(key, buff.String()).Result()
	if err != nil {
		return nil
	}

	index := elements - 1
	keyUserUsernameIndex := getKeyUsersUsernameListIndex(user.Username)
	keyUserUUIDIndex := getKeyUsersUUIDListIndex(user.UUID)

	err = u.redis.Set(keyUserUsernameIndex, fmt.Sprintf("%d", index), 0).Err()
	if err != nil {
		return err
	}

	err = u.redis.Set(keyUserUUIDIndex, fmt.Sprintf("%d", index), 0).Err()
	if err != nil {
		u.redis.Del(keyUserUsernameIndex)
		return err
	}

	return nil
}

func getKeyUsersUsernameListIndex(username string) string {
	return fmt.Sprintf("%s.%x", keyUsersUsernameListIndex, md5.Sum([]byte(username)))
}

func getKeyUsersUUIDListIndex(userUUID string) string {
	return fmt.Sprintf("%s.%s", keyUsersUUIDListIndex, userUUID)
}

func getKeyUsers() string {
	return keyUsers
}
