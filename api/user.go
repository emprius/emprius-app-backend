package api

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/genjidb/genji/document"
	"github.com/rs/zerolog/log"
)

// registerHandler handles the register request. It creates a new user in the database.
func (a *API) register(r *Request) (interface{}, error) {
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
	log.Debug().Msgf("adding user %+v", user)
	if err := a.database.Exec("INSERT INTO user VALUES ?", &user); err != nil {
		return nil, fmt.Errorf(ErrCouldNotInsertToDatabase.Error()+": %w", err)
	}
	return nil, nil
}

// login handles the login request. It returns a JWT token if the login is successful.
func (a *API) login(r *Request) (interface{}, error) {
	// Get the user name from the request body
	loginInfo := Login{}
	if err := json.Unmarshal(r.Data, &loginInfo); err != nil {
		return nil, ErrInvalidRequestBodyData
	}
	doc, err := a.database.QueryDocument("SELECT * FROM user WHERE email = ?", loginInfo.Email)
	if err != nil {
		return nil, ErrWrongLogin
	}

	user := db.User{}
	if err := document.StructScan(doc, &user); err != nil {
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}
	if !bytes.Equal(user.Password, hashPassword(loginInfo.Password)) {
		return nil, fmt.Errorf("failed to scan user: %w", err)
	}

	// Generate a new token with the user name as the subject
	token, err := a.makeToken(user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &token, nil
}

// refresh handles the refresh request. It returns a new JWT token.
func (a *API) refresh(r *Request) (interface{}, error) {
	// Generate a new token with the user name as the subject
	token, err := a.makeToken(r.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &token, nil
}

func (a *API) userProfile(r *Request) (interface{}, error) {
	stream, err := a.database.QueryDocument("SELECT * FROM user WHERE email = ?", r.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user profile: %w", err)
	}
	user := db.User{}
	if err := document.StructScan(stream, &user); err != nil {
		return nil, fmt.Errorf("failed to scan user profile: %w", err)
	}
	return &user, nil
}

func (a *API) userProfileUpdate(r *Request) (interface{}, error) {
	newUserInfo := UserProfile{}
	if err := json.Unmarshal(r.Data, &newUserInfo); err != nil {
		return nil, ErrInvalidRequestBodyData
	}
	doc, err := a.database.QueryDocument("SELECT * FROM user WHERE email = ?", r.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user profile: %w", err)
	}
	user := db.User{}
	if err := document.StructScan(doc, &user); err != nil {
		return nil, fmt.Errorf("failed to scan user profile: %w", err)
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
	if err := a.database.Exec(`UPDATE user SET name = ?, avatarHash = ?, location = ?,
	active = ?, password = ?, community = ? WHERE email = ?`, user.Name, user.AvatarHash, &user.Location,
		user.Active, user.Password, user.Community, user.Email); err != nil {
		return nil, fmt.Errorf(ErrCouldNotInsertToDatabase.Error()+": %w", err)
	}
	return &user, nil
}
