package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// registerHandler handles the register request. It creates a new user in the database.
func (a *API) registerHandler(r *Request) (interface{}, error) {
	userInfo := Register{}
	if err := json.Unmarshal(r.Data, &userInfo); err != nil {
		return nil, ErrInvalidRequestBodyData
	}
	if userInfo.RegisterAuthToken != a.registerAuthToken {
		return nil, ErrInvalidRegisterAuthToken
	}
	user := db.User{
		Email:    userInfo.UserEmail,
		Password: hashPassword(userInfo.Password),
		Name:     userInfo.Name,
		Active:   true,
		Rating:   50,
		Tokens:   1000,
	}
	if userInfo.Avatar != nil {
		image, err := a.addImage(userInfo.Name+"_avatar", userInfo.Avatar)
		if err != nil {
			return nil, fmt.Errorf("could not add image: %w", err)
		}
		user.AvatarHash = image.Hash
	}
	if userInfo.Location != nil {
		user.Location = *userInfo.Location
	}

	if err := a.addUser(&user); err != nil {
		return nil, fmt.Errorf("could not add user: %w", err)
	}

	// Generate a new token with the user name as the subject
	token, err := a.makeToken(user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &token, nil
}

func (a *API) addUser(u *db.User) error {
	log.Debug().Msgf("adding user %q", u.Email)
	_, err := a.database.UserService.InsertUser(context.Background(), u)
	if err != nil {
		return fmt.Errorf("could not insert user to database: %w", err)
	}
	return nil
}

// login handles the login request. It returns a JWT token if the login is successful.
func (a *API) loginHandler(r *Request) (interface{}, error) {
	// Get the user name from the request body
	loginInfo := Login{}
	if err := json.Unmarshal(r.Data, &loginInfo); err != nil {
		return nil, ErrInvalidRequestBodyData
	}
	user, err := a.database.UserService.GetUserByEmail(context.Background(), loginInfo.Email)
	if err != nil {
		return nil, ErrWrongLogin
	}
	if !bytes.Equal(user.Password, hashPassword(loginInfo.Password)) {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Generate a new token with the user name as the subject
	token, err := a.makeToken(user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &token, nil
}

// refresh handles the refresh request. It returns a new JWT token.
func (a *API) refreshHandler(r *Request) (interface{}, error) {
	// Generate a new token with the user name as the subject
	token, err := a.makeToken(r.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &token, nil
}

// usersHandler list the existing users.
func (a *API) usersHandler(r *Request) (interface{}, error) {
	users, err := a.database.UserService.GetAllUsers(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	userList := make([]db.User, len(users))
	for i, u := range users {
		userList[i] = *u
	}
	return &UsersWrapper{Users: userList}, nil
}

// getUserHandler handles GET /users/{id}
func (a *API) getUserHandler(r *Request) (interface{}, error) {
	userID, err := primitive.ObjectIDFromHex(r.Context.URLParam("id"))
	if err != nil {
		return nil, &HTTPError{
			Code:    404,
			Message: "user not found",
		}
	}

	user, err := a.database.UserService.GetUserByID(context.Background(), userID)
	if err != nil {
		return nil, &HTTPError{
			Code:    404,
			Message: "user not found",
		}
	}

	return user, nil
}

func (a *API) userByEmail(userID string) (*db.User, error) {
	user, err := a.database.UserService.GetUserByEmail(context.Background(), userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}
	return user, nil
}

func (a *API) userProfileHandler(r *Request) (interface{}, error) {
	return a.userByEmail(r.UserID)
}

func (a *API) userProfileUpdateHandler(r *Request) (interface{}, error) {
	newUserInfo := UserProfile{}
	if err := json.Unmarshal(r.Data, &newUserInfo); err != nil {
		return nil, ErrInvalidRequestBodyData
	}
	user, err := a.database.UserService.GetUserByEmail(context.Background(), r.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user profile: %w", err)
	}
	if newUserInfo.Name != "" {
		user.Name = newUserInfo.Name
	}
	if newUserInfo.Community != "" {
		user.Community = newUserInfo.Community
	}
	var avatar *db.Image
	if len(newUserInfo.Avatar) > 0 {
		avatar, err = a.addImage(user.Name+"_avatar", newUserInfo.Avatar)
		if err != nil {
			return nil, fmt.Errorf("could not add image: %w", err)
		}
		user.AvatarHash = avatar.Hash
	}
	if newUserInfo.Location != nil {
		user.Location = *newUserInfo.Location
	}
	if newUserInfo.Active != nil {
		user.Active = *newUserInfo.Active
	}
	if newUserInfo.Password != "" {
		user.Password = hashPassword(newUserInfo.Password)
	}
	update := bson.M{
		"name":       user.Name,
		"avatarHash": user.AvatarHash,
		"location":   user.Location,
		"active":     user.Active,
		"password":   user.Password,
		"community":  user.Community,
	}
	_, err = a.database.UserService.UpdateUser(context.Background(), user.ID, update)
	if err != nil {
		return nil, fmt.Errorf(ErrCouldNotInsertToDatabase.Error()+": %w", err)
	}
	return &user, nil
}

// getUserProfileByIDHandler handles GET /users/{id}/profile
func (a *API) getUserProfileByIDHandler(r *Request) (interface{}, error) {
	userID, err := primitive.ObjectIDFromHex(r.Context.URLParam("id"))
	if err != nil {
		return nil, &HTTPError{
			Code:    404,
			Message: "user not found",
		}
	}

	user, err := a.database.UserService.GetUserByID(context.Background(), userID)
	if err != nil {
		return nil, &HTTPError{
			Code:    404,
			Message: "user not found",
		}
	}

	// Return only public profile information
	return &UserProfile{
		Name:      user.Name,
		Community: user.Community,
		Location:  &user.Location,
		Active:    &user.Active,
		Avatar:    user.AvatarHash,
	}, nil
}
