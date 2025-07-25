package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/emprius/emprius-app-backend/types"
	"github.com/rs/zerolog/log"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RegisterCommunityRoutes registers all community-related routes to the provided router group
func (a *API) RegisterCommunityRoutes(r chi.Router) {
	// POST /communities
	log.Info().Msg("register route POST /communities")
	r.Post("/communities", a.routerHandler(a.createCommunityHandler))
	// GET /communities/{communityId}
	log.Info().Msg("register route GET /communities/{communityId}")
	r.Get("/communities/{communityId}", a.routerHandler(a.getCommunityHandler))
	// PUT /communities/{communityId}
	log.Info().Msg("register route PUT /communities/{communityId}")
	r.Put("/communities/{communityId}", a.routerHandler(a.updateCommunityHandler))
	// DELETE /communities/{communityId}
	log.Info().Msg("register route DELETE /communities/{communityId}")
	r.Delete("/communities/{communityId}", a.routerHandler(a.deleteCommunityHandler))
	// GET /communities/{communityId}/members
	log.Info().Msg("register route GET /communities/{communityId}/members")
	r.Get("/communities/{communityId}/members", a.routerHandler(a.getCommunityUsersHandler))
	// GET /communities/{communityId}/tools
	log.Info().Msg("register route GET /communities/{communityId}/tools")
	r.Get("/communities/{communityId}/tools", a.routerHandler(a.getCommunityToolsHandler))
	// POST /communities/{communityId}/members/{userId}
	log.Info().Msg("register route POST /communities/{communityId}/members/{userId}")
	r.Post("/communities/{communityId}/members/{userId}", a.routerHandler(a.inviteUserToCommunityHandler))
	// DELETE /communities/{communityId}/members
	log.Info().Msg("register route DELETE /communities/{communityId}/members")
	r.Delete("/communities/{communityId}/members", a.routerHandler(a.leaveCommunityHandler))
	// DELETE /communities/{communityId}/members/{userId}
	log.Info().Msg("register route DELETE /communities/{communityId}/members/{userId}")
	r.Delete("/communities/{communityId}/members/{userId}", a.routerHandler(a.leaveCommunityHandler))
	// GET /communities/invites
	log.Info().Msg("register route GET /communities/invites")
	r.Get("/communities/invites", a.routerHandler(a.getUserPendingInvitesHandler))
	// PUT /communities/invites/{inviteId}
	log.Info().Msg("register route PUT /communities/invites/{inviteId}")
	r.Put("/communities/invites/{inviteId}", a.routerHandler(a.updateInviteStatusHandler))
}

// CreateCommunityRequest represents the request to create a new community
type CreateCommunityRequest struct {
	Name  string         `json:"name"`
	Image types.HexBytes `json:"image,omitempty"`
}

// UpdateCommunityRequest represents the request to update a community
type UpdateCommunityRequest struct {
	Name  string         `json:"name,omitempty"`
	Image types.HexBytes `json:"image,omitempty"`
}

// CommunityResponse represents a community in API responses
type CommunityResponse struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Image        types.HexBytes `json:"image,omitempty"`
	OwnerID      string         `json:"ownerId"`
	MembersCount int64          `json:"membersCount"`
	ToolsCount   int64          `json:"toolsCount"`
}

// PaginatedCommunityResponse wraps community with pagination info
type PaginatedCommunityResponse struct {
	Communities []*CommunityResponse `json:"communities"`
	Pagination  PaginationInfo       `json:"pagination"`
}

// CommunityUserResponse represents a user in a community
type CommunityUserResponse struct {
	UserPreview
	Role db.CommunityRole `json:"role"`
}

// PaginatedCommunityUserResponse wraps community users with pagination info
type PaginatedCommunityUserResponse struct {
	Users      []*CommunityUserResponse `json:"users"`
	Pagination PaginationInfo           `json:"pagination"`
}

// CommunityInviteResponse represents a community invitation
type CommunityInviteResponse struct {
	ID          string    `json:"id"`
	CommunityID string    `json:"communityId"`
	UserID      string    `json:"userId"`
	InviterID   string    `json:"inviterId"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
	Community   struct {
		Name  string         `json:"name"`
		Image types.HexBytes `json:"image"`
	} `json:"community"`
}

// createCommunityHandler handles POST /communities
func (a *API) createCommunityHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	var req CreateCommunityRequest
	if err := json.Unmarshal(r.Data, &req); err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Validate request
	if req.Name == "" {
		return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("community name is required"))
	}

	// Get user ID
	userID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, ErrInvalidUserID.WithErr(err)
	}

	// Create community
	community, err := a.database.CommunityService.CreateCommunity(r.Context.Request.Context(), req.Name, req.Image, userID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Get member count (should be 1 for a new community - just the owner)
	membersCount, err := a.database.CommunityService.CountCommunityMembers(r.Context.Request.Context(), community.ID)
	if err != nil {
		// Log the error but don't fail the request
		log.Error().Err(err).Str("communityId", community.ID.Hex()).Msg("Failed to count community members")
		membersCount = 1 // Default to 1 (the owner) if count fails
	}

	// Get tool count (should be 0 for a new community)
	toolsCount, err := a.database.CommunityService.CountCommunityTools(r.Context.Request.Context(), community.ID)
	if err != nil {
		// Log the error but don't fail the request
		log.Error().Err(err).Str("communityId", community.ID.Hex()).Msg("Failed to count community tools")
		toolsCount = 0 // Default to 0 if count fails
	}

	// Return response
	return &CommunityResponse{
		ID:           community.ID.Hex(),
		Name:         community.Name,
		Image:        community.Image,
		OwnerID:      community.OwnerID.Hex(),
		MembersCount: membersCount,
		ToolsCount:   toolsCount,
	}, nil
}

// getCommunityHandler handles GET /communities/{communityId}
func (a *API) getCommunityHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get community ID from URL
	communityIDStr := chi.URLParam(r.Context.Request, "communityId")
	communityID, err := primitive.ObjectIDFromHex(communityIDStr)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get community with member count and tool count
	ctx := r.Context.Request.Context()
	community, membersCount, toolsCount, err := a.database.CommunityService.GetCommunityWithMemberCount(ctx, communityID)
	if err != nil {
		return nil, ErrCommunityNotFound.WithErr(err)
	}

	// Return response
	return &CommunityResponse{
		ID:           community.ID.Hex(),
		Name:         community.Name,
		Image:        community.Image,
		OwnerID:      community.OwnerID.Hex(),
		MembersCount: membersCount,
		ToolsCount:   toolsCount,
	}, nil
}

// updateCommunityHandler handles PUT /communities/{communityId}
func (a *API) updateCommunityHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get community ID from URL
	communityIDStr := chi.URLParam(r.Context.Request, "communityId")
	communityID, err := primitive.ObjectIDFromHex(communityIDStr)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Parse request
	var req UpdateCommunityRequest
	if err := json.Unmarshal(r.Data, &req); err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get community to check ownership
	community, err := a.database.CommunityService.GetCommunity(r.Context.Request.Context(), communityID)
	if err != nil {
		return nil, ErrCommunityNotFound.WithErr(err)
	}

	// Check if user is the owner
	userID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, ErrInvalidUserID.WithErr(err)
	}

	if community.OwnerID != userID {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("only the community owner can update the community"))
	}

	// Update community
	err = a.database.CommunityService.UpdateCommunity(r.Context.Request.Context(), communityID, req.Name, req.Image)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Get updated community with member count and tool count
	ctx := r.Context.Request.Context()
	updatedCommunity, membersCount, toolsCount, err := a.database.CommunityService.GetCommunityWithMemberCount(ctx, communityID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Return response
	return &CommunityResponse{
		ID:           updatedCommunity.ID.Hex(),
		Name:         updatedCommunity.Name,
		Image:        updatedCommunity.Image,
		OwnerID:      updatedCommunity.OwnerID.Hex(),
		MembersCount: membersCount,
		ToolsCount:   toolsCount,
	}, nil
}

// deleteCommunityHandler handles DELETE /communities/{communityId}
func (a *API) deleteCommunityHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get community ID from URL
	communityIDStr := chi.URLParam(r.Context.Request, "communityId")
	communityID, err := primitive.ObjectIDFromHex(communityIDStr)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get community to check ownership
	community, err := a.database.CommunityService.GetCommunity(r.Context.Request.Context(), communityID)
	if err != nil {
		return nil, ErrCommunityNotFound.WithErr(err)
	}

	// Check if user is the owner
	userID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, ErrInvalidUserID.WithErr(err)
	}

	if community.OwnerID != userID {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("only the community owner can delete the community"))
	}

	// Delete community
	err = a.database.CommunityService.DeleteCommunity(r.Context.Request.Context(), communityID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	return nil, nil
}

// getCommunityUsersHandler handles GET /communities/{communityId}/users
func (a *API) getCommunityUsersHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get community ID from URL
	communityIDStr := chi.URLParam(r.Context.Request, "communityId")
	communityID, err := primitive.ObjectIDFromHex(communityIDStr)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get page from query parameters
	page, pageSize, err := r.Context.GetPaginationParams()
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	term := *r.Context.GetSearchTerm()

	// Get community users
	users, total, err := a.database.CommunityService.GetCommunityUsers(r.Context.Request.Context(), communityID, page, term)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Convert to response format
	userList := make([]*CommunityUserResponse, len(users))
	for i, user := range users {
		// Find user's role in this community
		var role db.CommunityRole
		for _, comm := range user.Communities {
			if comm.ID == communityID {
				role = comm.Role
				break
			}
		}

		userList[i] = &CommunityUserResponse{
			UserPreview: *new(UserPreview).FromDBUserPreview(user),
			Role:        role,
		}
	}

	response := &PaginatedCommunityUserResponse{
		Users:      userList,
		Pagination: CalculatePagination(page, pageSize, total),
	}
	return response, nil
}

// inviteUserToCommunityHandler handles POST /communities/{communityId}/invite/{userId}
func (a *API) inviteUserToCommunityHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get community ID from URL
	communityIDStr := chi.URLParam(r.Context.Request, "communityId")
	communityID, err := primitive.ObjectIDFromHex(communityIDStr)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get user ID from URL
	userIDStr := chi.URLParam(r.Context.Request, "userId")
	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get inviter ID
	inviterID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, ErrInvalidUserID.WithErr(err)
	}

	// Check that user is not inviting themselves
	if userID == inviterID {
		return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("users cannot invite themselves to a community"))
	}

	// Create invitation
	invite, err := a.database.CommunityService.InviteUserToCommunity(r.Context.Request.Context(), communityID, userID, inviterID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Get community details
	community, err := a.database.CommunityService.GetCommunity(r.Context.Request.Context(), communityID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Return response
	return &CommunityInviteResponse{
		ID:          invite.ID.Hex(),
		CommunityID: invite.CommunityID.Hex(),
		UserID:      invite.UserID.Hex(),
		InviterID:   invite.InviterID.Hex(),
		Status:      string(invite.Status),
		CreatedAt:   invite.CreatedAt,
		Community: struct {
			Name  string         `json:"name"`
			Image types.HexBytes `json:"image"`
		}{
			Name:  community.Name,
			Image: community.Image,
		},
	}, nil
}

// leaveCommunityHandler handles DELETE /communities/{communityId}/members/{userId}
func (a *API) leaveCommunityHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get community ID from URL
	communityIDStr := chi.URLParam(r.Context.Request, "communityId")
	communityID, err := primitive.ObjectIDFromHex(communityIDStr)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get user ID from URL or use authenticated user's ID
	userIDStr := chi.URLParam(r.Context.Request, "userId")
	var userID primitive.ObjectID

	if userIDStr != "" {
		// If userId is provided in URL, use it (for admin operations)
		userID, err = primitive.ObjectIDFromHex(userIDStr)
		if err != nil {
			return nil, ErrInvalidRequestBodyData.WithErr(err)
		}

		// Check if authenticated user is the community owner
		authUserID, err := primitive.ObjectIDFromHex(r.UserID)
		if err != nil {
			return nil, ErrInvalidUserID.WithErr(err)
		}

		// Only allow removing other users if authenticated user is the owner
		if userID != authUserID {
			community, err := a.database.CommunityService.GetCommunity(r.Context.Request.Context(), communityID)
			if err != nil {
				return nil, ErrCommunityNotFound.WithErr(err)
			}

			if community.OwnerID != authUserID {
				return nil, ErrUnauthorized.WithErr(fmt.Errorf("only the community owner can remove other users"))
			}
		}
	} else {
		// If no userId provided, use the authenticated user's ID
		userID, err = primitive.ObjectIDFromHex(r.UserID)
		if err != nil {
			return nil, ErrInvalidUserID.WithErr(err)
		}
	}

	// Leave community
	err = a.database.CommunityService.LeaveCommunity(r.Context.Request.Context(), communityID, userID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	return nil, nil
}

// getUserPendingInvitesHandler handles GET /users/invites
func (a *API) getUserPendingInvitesHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get user ID
	userID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, ErrInvalidUserID.WithErr(err)
	}

	// Get pending invites with community details
	invites, err := a.database.CommunityService.GetUserPendingInvitesWithDetails(r.Context.Request.Context(), userID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Convert to response format
	response := make([]CommunityInviteResponse, len(invites))
	for i, invite := range invites {
		response[i] = CommunityInviteResponse{
			ID:          invite.ID.Hex(),
			CommunityID: invite.CommunityID.Hex(),
			UserID:      invite.UserID.Hex(),
			InviterID:   invite.InviterID.Hex(),
			Status:      string(invite.Status),
			CreatedAt:   invite.CreatedAt,
			Community: struct {
				Name  string         `json:"name"`
				Image types.HexBytes `json:"image"`
			}{
				Name:  invite.Community.Name,
				Image: invite.Community.Image,
			},
		}
	}

	return response, nil
}

// InviteStatusUpdateRequest represents the request to update an invite status
type InviteStatusUpdateRequest struct {
	Status string `json:"status"` // "ACCEPTED", "REJECTED", or "CANCELED"
}

// updateInviteStatusHandler handles PUT /invites/{inviteId}
func (a *API) updateInviteStatusHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get invite ID from URL
	inviteIDStr := chi.URLParam(r.Context.Request, "inviteId")
	inviteID, err := primitive.ObjectIDFromHex(inviteIDStr)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get user ID
	userID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, ErrInvalidUserID.WithErr(err)
	}

	// Parse request
	var req InviteStatusUpdateRequest
	if err := json.Unmarshal(r.Data, &req); err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Validate status
	if req.Status != "ACCEPTED" && req.Status != "REJECTED" && req.Status != "CANCELED" {
		return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("invalid status: must be 'ACCEPTED', 'REJECTED', or 'CANCELED'"))
	}

	// Update invite status
	switch req.Status {
	case "ACCEPTED":
		err = a.database.CommunityService.AcceptInvite(r.Context.Request.Context(), inviteID, userID)
	case "REJECTED":
		err = a.database.CommunityService.RejectInvite(r.Context.Request.Context(), inviteID, userID)
	case "CANCELED":
		// Only the inviter can cancel an invite
		err = a.database.CommunityService.CancelInvite(r.Context.Request.Context(), inviteID, userID)
	}

	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Get updated invites with community details
	invites, err := a.database.CommunityService.GetUserPendingInvitesWithDetails(r.Context.Request.Context(), userID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Convert to response format
	response := make([]CommunityInviteResponse, len(invites))
	for i, invite := range invites {
		response[i] = CommunityInviteResponse{
			ID:          invite.ID.Hex(),
			CommunityID: invite.CommunityID.Hex(),
			UserID:      invite.UserID.Hex(),
			InviterID:   invite.InviterID.Hex(),
			Status:      string(invite.Status),
			CreatedAt:   invite.CreatedAt,
			Community: struct {
				Name  string         `json:"name"`
				Image types.HexBytes `json:"image"`
			}{
				Name:  invite.Community.Name,
				Image: invite.Community.Image,
			},
		}
	}

	return response, nil
}

// getUserCommunitiesHandler handles GET /users/{userId}/communities
func (a *API) getUserCommunitiesHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get user ID from URL
	userIDStr := chi.URLParam(r.Context.Request, "userId")
	userID, err := primitive.ObjectIDFromHex(userIDStr)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Use access control method to check if user can be accessed
	_, err = a.GetUserByIDWithAccessControl(r, userID)
	if err != nil {
		return nil, err
	}

	// Get page from query parameters
	page, pageSize, err := r.Context.GetPaginationParams()
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	term := *r.Context.GetSearchTerm()

	// Get communities for the user with member counts and tool counts
	ctx := r.Context.Request.Context()
	communities,
		memberCounts,
		toolCounts,
		totalCommunities,
		err := a.database.CommunityService.GetUserCommunitiesWithMemberCount(ctx, userID, page, term)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Convert to response format
	communitiesResponse := make([]*CommunityResponse, len(communities))
	for i, community := range communities {
		communitiesResponse[i] = &CommunityResponse{
			ID:           community.ID.Hex(),
			Name:         community.Name,
			Image:        community.Image,
			OwnerID:      community.OwnerID.Hex(),
			MembersCount: memberCounts[community.ID],
			ToolsCount:   toolCounts[community.ID],
		}
	}

	response := &PaginatedCommunityResponse{
		Communities: communitiesResponse,
		Pagination:  CalculatePagination(page, pageSize, totalCommunities),
	}
	return response, nil
}

// addToolToCommunityHandler handles adding a tool to a community
// This is part of the tool update process in editToolHandler
func (a *API) addToolToCommunity(ctx context.Context, toolID int64, communityIDs []string) error {
	// Get existing tool to get its current communities
	tool, err := a.database.ToolService.GetToolByID(ctx, toolID)
	if err != nil {
		return err
	}

	// Convert string IDs to ObjectIDs
	newCommunities := make([]primitive.ObjectID, 0, len(communityIDs))
	for _, idStr := range communityIDs {
		id, err := primitive.ObjectIDFromHex(idStr)
		if err != nil {
			return ErrInvalidRequestBodyData.WithErr(fmt.Errorf("invalid community ID: %s", idStr))
		}
		newCommunities = append(newCommunities, id)
	}

	// Find communities to add and remove
	currentCommunities := tool.Communities
	toAdd := make([]primitive.ObjectID, 0)
	toRemove := make([]primitive.ObjectID, 0)

	// Find communities to add
	for _, newComm := range newCommunities {
		found := false
		for _, currentComm := range currentCommunities {
			if newComm == currentComm {
				found = true
				break
			}
		}
		if !found {
			toAdd = append(toAdd, newComm)
		}
	}

	// Find communities to remove
	for _, currentComm := range currentCommunities {
		found := false
		for _, newComm := range newCommunities {
			if currentComm == newComm {
				found = true
				break
			}
		}
		if !found {
			toRemove = append(toRemove, currentComm)
		}
	}

	// Add tool to new communities
	for _, communityID := range toAdd {
		err := a.database.CommunityService.AddToolToCommunity(ctx, toolID, communityID)
		if err != nil {
			return err
		}
	}

	// Remove tool from old communities
	for _, communityID := range toRemove {
		err := a.database.CommunityService.RemoveToolFromCommunity(ctx, toolID, communityID)
		if err != nil {
			return err
		}
	}

	return nil
}

// getCommunityToolsHandler handles GET /communities/{communityId}/tools
func (a *API) getCommunityToolsHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	// Get community ID from URL
	communityIDStr := chi.URLParam(r.Context.Request, "communityId")
	communityID, err := primitive.ObjectIDFromHex(communityIDStr)
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Check if community exists
	exists, err := a.database.CommunityService.CommunityExists(r.Context.Request.Context(), communityID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}
	if !exists {
		return nil, ErrCommunityNotFound.WithErr(fmt.Errorf("community with id %s not found", communityIDStr))
	}

	// Get pagination parameters
	page, pageSize, err := r.Context.GetPaginationParams()
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get search term
	searchTerm := *r.Context.GetSearchTerm()

	// Get paginated tools for the community
	dbTools, total, err := a.database.CommunityService.GetCommunityToolsPaginated(
		r.Context.Request.Context(),
		communityID,
		page,
		pageSize,
		searchTerm,
	)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	return a.getToolListPaginatedResponse(dbTools, page, pageSize, total), nil
}
