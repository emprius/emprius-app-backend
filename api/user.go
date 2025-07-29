package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/emprius/emprius-app-backend/notifications"

	"github.com/emprius/emprius-app-backend/notifications/mailtemplates"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/emprius/emprius-app-backend/types"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RegisterUserRoutes registers all user-related routes to the provided router group
func (a *API) RegisterUserRoutes(r chi.Router) {
	// GET /profile
	log.Info().Msg("register route GET /profile")
	r.Get("/profile", a.routerHandler(a.userProfileHandler))
	// POST /profile
	log.Info().Msg("register route POST /profile")
	r.Post("/profile", a.routerHandler(a.userProfileUpdateHandler))
	// POST /profile/invites
	log.Info().Msg("register route POST /profile/invites")
	r.Post("/profile/invites", a.routerHandler(a.userInviteCodesHandler))
	// GET /profile/pendings
	log.Info().Msg("register route GET /profile/pendings")
	r.Get("/profile/pendings", a.routerHandler(a.HandleCountPendingActions))
	// POST /profile/notifications
	log.Info().Msg("register route POST /profile/notifications")
	r.Post("/profile/notifications", a.routerHandler(a.userNotificationPreferencesUpdateHandler))
	// GET /users
	log.Info().Msg("register route GET /users")
	r.Get("/users", a.routerHandler(a.usersHandler))
	// GET /users/{id}
	log.Info().Msg("register route GET /users/{id}")
	r.Get("/users/{id}", a.routerHandler(a.getUserHandler))
	// GET /users/{id}/ratings
	log.Info().Msg("register route GET /users/{id}/ratings")
	r.Get("/users/{id}/ratings", a.routerHandler(a.HandleGetUserRatings))
	// GET /users/{userId}/communities
	log.Info().Msg("register route GET /users/{userId}/communities")
	r.Get("/users/{userId}/communities", a.routerHandler(a.getUserCommunitiesHandler))
	// GET /refresh
	log.Info().Msg("register route GET /refresh")
	r.Get("/refresh", a.routerHandler(a.refreshHandler))
}

// RegisterPublicUserRoutes registers all public user-related routes to the provided router group
func (a *API) RegisterPublicUserRoutes(r chi.Router) {
	// POST /login
	log.Info().Msg("register route POST /login")
	r.Post("/login", a.routerHandler(a.loginHandler))
	// POST /register
	log.Info().Msg("register route POST /register")
	r.Post("/register", a.routerHandler(a.registerHandler))
}

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

	// check the name is not empty
	if userInfo.Name == "" {
		return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("name is empty"))
	}

	// check the name is not empty
	if userInfo.Community == "" {
		return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("community is empty"))
	}

	// check the password is correct format
	if len(userInfo.Password) < 8 {
		return nil, ErrPasswordTooShort
	}

	// check the email is correct format
	if !notifications.ValidEmail(userInfo.UserEmail) {
		return nil, ErrMalformedEmail
	}

	// check the location is set
	if userInfo.Location == nil {
		return nil, ErrLocationNotSet
	}

	// Add additional contacts if provided
	if userInfo.AdditionalContacts != nil {
		err := userInfo.AdditionalContacts.Validate()
		if err != nil {
			return nil, err
		}
	}

	// Generate a random salt for the password
	randomSalt, err := generateRandomSalt()
	if err != nil {
		return nil, ErrInternalServerError.WithErr(fmt.Errorf("failed to generate salt: %w", err))
	}

	user := db.User{
		Email:                   userInfo.UserEmail,
		Password:                hashPassword(userInfo.Password, randomSalt),
		Salt:                    randomSalt,
		Name:                    userInfo.Name,
		Community:               userInfo.Community,
		Location:                userInfo.Location.ToDBLocation(),
		Active:                  true,
		Rating:                  50,
		Karma:                   200,
		Tokens:                  1000,
		NotificationPreferences: db.GetDefaultNotificationPreferences(),
		AdditionalContacts:      userInfo.AdditionalContacts,
	}

	if userInfo.Avatar != nil {
		image, err := a.addImage(userInfo.Name+"_avatar", userInfo.Avatar)
		if err != nil {
			return nil, ErrInvalidImageFormat.WithErr(err)
		}
		user.AvatarHash = image.Hash
	}

	id, err := a.addUser(&user)

	// Generate and update obfuscated location after user is created and ID is assigned
	if userInfo.Location != nil {
		// Update the user with obfuscated location
		obfuscatedLocation := db.ObfuscateLocation(user.Location, id, randomSalt)
		update := bson.M{"obfuscatedLocation": obfuscatedLocation}
		_, err := a.database.UserService.UpdateUser(context.Background(), id, update)
		if err != nil {
			log.Error().Err(err).Str("userId", id.Hex()).Msg("Failed to update obfuscated location")
			// Continue even if update fails
		}
	}
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

	// Generate initial invite codes for the new user
	codeCount := a.maxInviteCodes
	_, err = a.database.InviteCodeService.CreateInviteCodes(context.Background(), id, codeCount)
	if err != nil {
		log.Error().Err(err).Str("userId", id.Hex()).Msg("Failed to generate initial invite codes")
		// Continue even if code generation fails
	}

	// Generate a new token with the user's ObjectID
	token, err := a.makeToken(id.Hex())
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// send the welcome email
	if err := a.sendMail(r.Context.Request.Context(), user.Email, mailtemplates.WelcomeMailNotification,
		struct {
			AppName string
			AppUrl  string
			LogoURL string
		}{mailtemplates.AppName, mailtemplates.AppUrl, mailtemplates.LogoURL},
	); err != nil {
		log.Warn().Err(err).Msg("could not send welcome email")
		// Continue even if email cannot be sent
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

	// Check if password is empty (password recovery scenario)
	if len(user.Password) == 0 {
		// Password recovery: set the provided password as the new password
		// Use existing salt or set default salt if user doesn't have one
		salt := user.Salt
		if salt == "" {
			salt = passwordSalt
		}
		newPasswordHash := hashPassword(loginInfo.Password, salt)
		update := bson.M{
			"password": newPasswordHash,
			"salt":     salt,
		}
		_, err = a.database.UserService.UpdateUser(context.Background(), user.ID, update)
		if err != nil {
			log.Error().Err(err).Str("userId", user.ID.Hex()).Msg("Failed to update password during recovery")
			return nil, ErrInternalServerError.WithErr(fmt.Errorf("failed to update password: %w", err))
		}

		log.Info().Str("userId", user.ID.Hex()).Str("email", user.Email).Msg("Password recovery completed successfully")
		// Continue with normal login flow after setting password
	} else {
		// Normal login: compare passwords
		// Use existing salt or default salt for backward compatibility
		salt := user.Salt
		if salt == "" {
			salt = passwordSalt
		}
		if !bytes.Equal(user.Password, hashPassword(loginInfo.Password, salt)) {
			return nil, ErrWrongLogin
		}
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
// If username query parameter is provided, it will search for users with partial name match.
func (a *API) usersHandler(r *Request) (interface{}, error) {
	// Get pagination parameters
	page, pageSize, err := r.Context.GetPaginationParams()
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	searchTerm := *r.Context.GetSearchTerm()

	var users []*db.User
	var total int64

	users, total, err = a.database.UserService.GetUsers(r.Context.Request.Context(), searchTerm, page)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	userList := []*User{}
	for _, u := range users {
		userList = append(userList, new(User).FromDBUser(u, false, false))
	}

	// Return users with pagination info
	response := &UsersWrapper{
		Users:      userList,
		Pagination: CalculatePagination(page, pageSize, total),
	}
	return response, nil
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

	// Use access control method to check if user can be accessed
	u, requestingUserID, err := a.GetUserByIDWithAccessControl(r, userID)
	if err != nil {
		return nil, err
	}

	// Determine if we should include private data (including AdditionalContacts)
	includeAdditionalContacts := false

	// Case 1: Requesting user is the same as queried user (profile case)
	if *requestingUserID == userID {
		includeAdditionalContacts = true
	} else {
		// Case 2: Check if users have accepted bookings together
		hasAcceptedBooking, err := a.database.BookingService.HasAcceptedBookingBetweenUsers(
			r.Context.Request.Context(),
			*requestingUserID,
			userID,
		)
		if err != nil {
			log.Error().Err(err).
				Str("requestingUserId", requestingUserID.Hex()).
				Str("queriedUserId", userID.Hex()).
				Msg("Failed to check accepted bookings between users")
			// Continue without private data rather than failing
		} else {
			includeAdditionalContacts = hasAcceptedBooking
		}
	}

	return new(User).FromDBUser(u, false, includeAdditionalContacts), nil
}

// validateObjectID checks if a string is a valid MongoDB ObjectID
func validateObjectID(id string) error {
	if _, err := primitive.ObjectIDFromHex(id); err != nil {
		return ErrInvalidUserID.WithErr(err)
	}
	return nil
}

func (a *API) getUserByID(userID string) (*db.User, error) {
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
	dbUser, err := a.getDBUserByID(r.UserID)
	if err != nil {
		return nil, err
	}

	// Create API user from DB user with real location and private data (true parameters)
	user := new(User).FromDBUser(dbUser, true, true)

	// Get user's unused invite codes
	objID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, ErrInvalidUserID.WithErr(err)
	}

	// Get notification preferences with proper merging of defaults
	// This ensures that new notifications types are included for existing users that has not set them yet
	notificationPrefs, err := a.database.UserService.GetNotificationPreferences(context.Background(), objID)
	if err != nil {
		log.Error().Err(err).Str("userId", r.UserID).Msg("Failed to get notification preferences")
		// Continue with default preferences rather than failing
		notificationPrefs = db.GetDefaultNotificationPreferences()
	}
	user.NotificationPreferences = notificationPrefs

	inviteCodes, err := a.database.InviteCodeService.GetUnusedInviteCodesByOwnerID(context.Background(), objID)
	if err != nil {
		log.Error().Err(err).Str("userId", r.UserID).Msg("Failed to get invite codes")
		// Continue even if getting invite codes fails
	} else {
		// Convert DB invite codes to simplified API invite codes
		user.InviteCodes = make([]*SimpleInviteCode, len(inviteCodes))
		for i, dbInviteCode := range inviteCodes {
			user.InviteCodes[i] = &SimpleInviteCode{
				Code:      dbInviteCode.Code,
				CreatedOn: dbInviteCode.CreatedOn,
			}
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

	// Convert DB invite codes to simplified API invite codes
	apiCodes := make([]*SimpleInviteCode, len(newCodes))
	for i, dbCode := range newCodes {
		apiCodes[i] = &SimpleInviteCode{
			Code:      dbCode.Code,
			CreatedOn: dbCode.CreatedOn,
		}
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
		// Generate obfuscated location
		user.ObfuscatedLocation = db.ObfuscateLocation(user.Location, user.ID, user.Salt)
	}

	if newUserInfo.Active != nil {
		user.Active = *newUserInfo.Active
	}

	if newUserInfo.Password != "" {
		// If password is being changed, require the actual password
		if newUserInfo.ActualPassword == "" {
			return nil, ErrActualPasswordRequired
		}

		// Use existing salt or default salt for backward compatibility
		if user.Salt == "" {
			user.Salt = passwordSalt
		}

		// Verify the actual password matches the stored password
		if !bytes.Equal(user.Password, hashPassword(newUserInfo.ActualPassword, user.Salt)) {
			return nil, ErrInvalidActualPassword
		}

		user.Password = hashPassword(newUserInfo.Password, user.Salt)
	}

	if newUserInfo.AdditionalContacts != nil {
		err := newUserInfo.AdditionalContacts.Validate()
		if err != nil {
			return nil, err
		}
		user.AdditionalContacts = newUserInfo.AdditionalContacts
	}

	update := bson.M{
		"name":               user.Name,
		"avatarHash":         user.AvatarHash,
		"location":           user.Location,
		"obfuscatedLocation": user.ObfuscatedLocation,
		"active":             user.Active,
		"password":           user.Password,
		"salt":               user.Salt,
		"community":          user.Community,
		"bio":                user.Bio,
		"lastSeen":           time.Now(), // Update lastSeen when profile is updated
		"additionalContacts": user.AdditionalContacts,
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

// HandleCountPendingActions handles GET /profile/pendings
func (a *API) HandleCountPendingActions(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}
	uID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	pending, err := a.database.BookingService.CountPendingActions(r.Context.Request.Context(), uID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Get pending community invites
	pendingInvitesCount, err := a.database.CommunityService.CountUserPendingInvites(r.Context.Request.Context(), uID)
	if err != nil {
		log.Error().Err(err).Str("userId", r.UserID).Msg("Failed to count pending invites")
		// Continue even if count fails, just set to 0
		pendingInvitesCount = 0
	}

	// Create response with all pending counts
	response := &PendingActionsResponse{
		PendingRatingsCount:  pending.PendingRatingsCount,
		PendingRequestsCount: pending.PendingRequestsCount,
		PendingInvitesCount:  pendingInvitesCount,
	}

	return response, nil
}

// HandleGetUserRatings handles GET /users/{id}/ratings
// Returns a unified list of all ratings (both submitted and received) for a user
func (a *API) HandleGetUserRatings(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get user ID from URL
	userID, err := primitive.ObjectIDFromHex(chi.URLParam(r.Context.Request, "id"))
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Use access control method to check if user can be accessed
	_, _, err = a.GetUserByIDWithAccessControl(r, userID)
	if err != nil {
		return nil, err
	}

	// Get pagination parameters
	page, pageSize, err := r.Context.GetPaginationParams()
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get unified ratings
	unifiedRatings, total, err := a.database.BookingService.GetRatingsByUserId(
		r.Context.Request.Context(),
		userID,
		page,
		pageSize,
	)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	if unifiedRatings == nil {
		// Return empty array instead of nil
		unifiedRatings = make([]*db.UnifiedRating, 0)
	}

	return a.getUnifiedRatingsPaginatedResponse(unifiedRatings, page, pageSize, total), nil
}

// userNotificationPreferencesUpdateHandler handles POST /profile/notifications
func (a *API) userNotificationPreferencesUpdateHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	var preferences NotificationPreferences
	if err := json.Unmarshal(r.Data, &preferences); err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	objID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, ErrInvalidUserID.WithErr(err)
	}

	// Validate that only known notification types are being set
	for key := range preferences {
		if !types.IsValidNotificationType(key) {
			return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("unknown notification type: %s", key))
		}
	}

	err = a.database.UserService.UpdateNotificationPreferences(context.Background(), objID, map[string]bool(preferences))
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Return the updated preferences
	updatedPreferences, err := a.database.UserService.GetNotificationPreferences(context.Background(), objID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	return NotificationPreferences(updatedPreferences), nil
}

// GetUserByIDWithAccessControl Get user by ID with access control
// This method checks if the requesting user has access to the user being requested.
func (a *API) GetUserByIDWithAccessControl(r *Request, userID primitive.ObjectID) (*db.User, *primitive.ObjectID, error) {
	// Get requesting user ID for access control
	var requestingUserID primitive.ObjectID
	var err error
	if r.UserID != "" {
		requestingUserID, err = primitive.ObjectIDFromHex(r.UserID)
		if err != nil {
			return nil, nil, ErrInvalidUserID.WithErr(err)
		}
	}

	// Use access control method to check if user can be accessed
	user, err := a.database.UserService.GetUserByIDWithAccessControl(r.Context.Request.Context(), userID, requestingUserID)
	if err != nil {
		return nil, nil, ErrUserNotFound.WithErr(err)
	}

	return user, &requestingUserID, nil
}
