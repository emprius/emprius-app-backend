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
		return nil, ErrInvalidRequestBodyData.WithErr(err)
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
			return nil, ErrInvalidImageFormat.WithErr(err)
		}
		user.AvatarHash = image.Hash
	}
	if userInfo.Location != nil {
		user.Location = userInfo.Location.ToDBLocation()
	}

	id, err := a.addUser(&user)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}
	// Generate a new token with the user's ObjectID
	token, err := a.makeToken(id.Hex())
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	return &token, nil
}

func (a *API) addUser(u *db.User) (primitive.ObjectID, error) {
	log.Debug().Msgf("adding user %q with location %v", u.Email, u.Location)
	r, err := a.database.UserService.InsertUser(context.Background(), u)
	if err != nil {
		return [12]byte{}, fmt.Errorf("could not insert user to database: %w", err)
	}
	return r.InsertedID.(primitive.ObjectID), nil
}

// login handles the login request. It returns a JWT token if the login is successful.
func (a *API) loginHandler(r *Request) (interface{}, error) {
	// Get the user name from the request body
	loginInfo := Login{}
	if err := json.Unmarshal(r.Data, &loginInfo); err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}
	user, err := a.database.UserService.GetUserByEmail(context.Background(), loginInfo.Email)
	if err != nil {
		return nil, ErrWrongLogin
	}
	if !bytes.Equal(user.Password, hashPassword(loginInfo.Password)) {
		return nil, ErrWrongLogin
	}

	// Generate a new token with the user's ObjectID
	token, err := a.makeToken(user.ID.Hex())
	if err != nil {
		return nil, ErrInternalServerError.WithErr(fmt.Errorf("failed to generate token: %w", err))
	}

	return &token, nil
}

// refresh handles the refresh request. It returns a new JWT token.
func (a *API) refreshHandler(r *Request) (interface{}, error) {
	// Generate a new token with the user name as the subject
	token, err := a.makeToken(r.UserID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	return &token, nil
}

// usersHandler list the existing users with pagination.
func (a *API) usersHandler(r *Request) (interface{}, error) {
	page, err := r.Context.GetPage()
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}
	users, err := a.database.UserService.GetAllUsers(r.Context.Request.Context(), page)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}
	userList := []*User{}
	for _, u := range users {
		userList = append(userList, new(User).FromDBUser(u))
	}
	return &UsersWrapper{Users: userList}, nil
}

// getUserHandler handles GET /users/{id}
func (a *API) getUserHandler(r *Request) (interface{}, error) {
	idParam := r.Context.URLParam("id")
	if idParam == nil {
		return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("missing id"))
	}
	userID, err := primitive.ObjectIDFromHex(idParam[0])
	if err != nil {
		return nil, ErrUserNotFound.WithErr(fmt.Errorf("invalid user id format: %s", r.Context.URLParam("id")))
	}

	user, err := a.database.UserService.GetUserByID(context.Background(), userID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}

	return new(User).FromDBUser(user), nil
}

// validateObjectID checks if a string is a valid MongoDB ObjectID
func validateObjectID(id string) error {
	if _, err := primitive.ObjectIDFromHex(id); err != nil {
		return ErrInvalidUserID.WithErr(err)
	}
	return nil
}

func (a *API) getUserByID(userID string) (*User, error) {
	if err := validateObjectID(userID); err != nil {
		return nil, err
	}
	objID, _ := primitive.ObjectIDFromHex(userID) // Safe to ignore error as we already validated
	user, err := a.database.UserService.GetUserByID(context.Background(), objID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}
	return new(User).FromDBUser(user), nil
}

func (a *API) getDBUserByID(userID string) (*db.User, error) {
	if err := validateObjectID(userID); err != nil {
		return nil, err
	}
	objID, _ := primitive.ObjectIDFromHex(userID) // Safe to ignore error as we already validated
	user, err := a.database.UserService.GetUserByID(context.Background(), objID)
	if err != nil {
		return nil, ErrUserNotFound.WithErr(err)
	}
	return user, nil
}

func (a *API) userProfileHandler(r *Request) (interface{}, error) {
	return a.getUserByID(r.UserID)
}

func (a *API) userProfileUpdateHandler(r *Request) (interface{}, error) {
	newUserInfo := UserProfile{}
	if err := json.Unmarshal(r.Data, &newUserInfo); err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Check if the request contains an email field
	var requestMap map[string]interface{}
	if err := json.Unmarshal(r.Data, &requestMap); err == nil {
		if email, ok := requestMap["email"]; ok && email != "" {
			// Email change attempt detected
			return nil, ErrEmailChangeNotAllowed
		}
	}
	user, err := a.getDBUserByID(r.UserID)
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
		user.Location = newUserInfo.Location.ToDBLocation()
	}
	if newUserInfo.Active != nil {
		user.Active = *newUserInfo.Active
	}
	if newUserInfo.Password != "" {
		// If password is being changed, require the actual password
		if newUserInfo.ActualPassword == "" {
			return nil, ErrActualPasswordRequired
		}

		// Verify the actual password matches the stored password
		if !bytes.Equal(user.Password, hashPassword(newUserInfo.ActualPassword)) {
			return nil, ErrInvalidActualPassword
		}

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
		return nil, ErrCouldNotInsertToDatabase.WithErr(err)
	}
	newUser, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user profile: %w", err)
	}
	return newUser, nil
}
