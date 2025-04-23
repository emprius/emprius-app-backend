package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

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

	// Check if using master register token or invite code
	var inviteCode *db.InviteCode
	var err error

	if userInfo.RegisterAuthToken == a.registerAuthToken {
		// Using master register token, proceed without invite code
		log.Debug().Msg("Registration using master token")
	} else {
		// Check if the token is a valid invite code
		inviteCode, err = a.database.InviteCodeService.GetInviteCodeByCode(context.Background(), userInfo.RegisterAuthToken)
		if err != nil {
			return nil, ErrInvalidInviteCode.WithErr(err)
		}

		// Check if the invite code has already been used
		if inviteCode.UsedByID != nil {
			return nil, ErrInviteCodeAlreadyUsed
		}
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

	// If using an invite code, mark it as used
	if inviteCode != nil {
		err = a.database.InviteCodeService.MarkCodeAsUsed(context.Background(), inviteCode.Code, id)
		if err != nil {
			log.Error().Err(err).Str("code", inviteCode.Code).Str("userId", id.Hex()).Msg("Failed to mark invite code as used")
			// Continue even if marking as used fails
		}
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

	// Update LastSeen timestamp
	now := time.Now()
	update := bson.M{"lastSeen": now}
	_, err = a.database.UserService.UpdateUser(context.Background(), user.ID, update)
	if err != nil {
		log.Error().Err(err).Str("userId", user.ID.Hex()).Msg("Failed to update LastSeen timestamp")
		// Continue even if update fails
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

	return a.getUserByID(userID.Hex())
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

	// Create API user from DB user
	apiUser := new(User).FromDBUser(user)

	// Get rating count (number of ratings received by this user)
	filter := bson.M{
		"rateeId": objID,
		"raterId": bson.M{"$ne": objID}, // exclude self-ratings
	}

	ratingCount, err := a.database.Database.Collection("ratings").CountDocuments(context.Background(), filter)
	if err != nil {
		log.Error().Err(err).Str("userId", userID).Msg("Failed to count user ratings")
		// Continue even if count fails, just set to 0
		apiUser.RatingCount = 0
	} else {
		apiUser.RatingCount = int(ratingCount)
	}

	return apiUser, nil
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
	user, err := a.getUserByID(r.UserID)
	if err != nil {
		return nil, err
	}

	// Get user's invite codes
	objID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, ErrInvalidUserID.WithErr(err)
	}

	inviteCodes, err := a.database.InviteCodeService.GetAllInviteCodesByOwnerID(context.Background(), objID)
	if err != nil {
		log.Error().Err(err).Str("userId", r.UserID).Msg("Failed to get invite codes")
		// Continue even if getting invite codes fails
	} else {
		// Convert DB invite codes to API invite codes
		user.InviteCodes = make([]*InviteCode, len(inviteCodes))
		for i, dbInviteCode := range inviteCodes {
			user.InviteCodes[i] = new(InviteCode).FromDBInviteCode(dbInviteCode)
		}
	}

	return user, nil
}

// userInviteCodesHandler handles POST /profile/invites
func (a *API) userInviteCodesHandler(r *Request) (interface{}, error) {
	// Get user ID
	objID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, ErrInvalidUserID.WithErr(err)
	}

	// Check if user already has unused invite codes
	unusedCodes, err := a.database.InviteCodeService.GetUnusedInviteCodesByOwnerID(context.Background(), objID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	if len(unusedCodes) > 0 {
		return nil, ErrHasUnusedInviteCodes
	}

	// Check when the user last requested invite codes
	lastRequestTime, err := a.database.InviteCodeService.GetLastCodeRequestTime(context.Background(), objID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// If the user has requested codes before, check if enough time has passed
	if lastRequestTime != nil {
		cooldownPeriod := time.Duration(a.inviteCodeCooldown) * 24 * time.Hour
		if time.Since(*lastRequestTime) < cooldownPeriod {
			return nil, ErrTooManyInviteCodeRequests
		}
	}

	// Use the configured number of codes to generate
	codeCount := a.maxInviteCodes

	// Generate new invite codes
	newCodes, err := a.database.InviteCodeService.CreateInviteCodes(context.Background(), objID, codeCount)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Convert DB invite codes to API invite codes
	apiCodes := make([]*InviteCode, len(newCodes))
	for i, dbCode := range newCodes {
		apiCodes[i] = new(InviteCode).FromDBInviteCode(dbCode)
	}

	return apiCodes, nil
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
	// Update bio if provided
	if newUserInfo.Bio != "" {
		user.Bio = newUserInfo.Bio
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
		"bio":        user.Bio,
		"lastSeen":   time.Now(), // Update lastSeen when profile is updated
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
