package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CreateCommunityRequest represents the request to create a new community
type CreateCommunityRequest struct {
	Name      string `json:"name"`
	ImageHash []byte `json:"imageHash,omitempty"`
}

// UpdateCommunityRequest represents the request to update a community
type UpdateCommunityRequest struct {
	Name      string `json:"name,omitempty"`
	ImageHash []byte `json:"imageHash,omitempty"`
}

// CommunityResponse represents a community in API responses
type CommunityResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ImageHash []byte `json:"imageHash,omitempty"`
	OwnerID   string `json:"ownerId"`
}

// CommunityUserResponse represents a user in a community
type CommunityUserResponse struct {
	ID   string           `json:"id"`
	Name string           `json:"name"`
	Role db.CommunityRole `json:"role"`
}

// CommunityInviteResponse represents a community invitation
type CommunityInviteResponse struct {
	ID          string `json:"id"`
	CommunityID string `json:"communityId"`
	UserID      string `json:"userId"`
	InviterID   string `json:"inviterId"`
	Status      string `json:"status"`
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
	community, err := a.database.CommunityService.CreateCommunity(r.Context.Request.Context(), req.Name, req.ImageHash, userID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Return response
	return &CommunityResponse{
		ID:        community.ID.Hex(),
		Name:      community.Name,
		ImageHash: community.ImageHash,
		OwnerID:   community.OwnerID.Hex(),
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

	// Get community
	community, err := a.database.CommunityService.GetCommunity(r.Context.Request.Context(), communityID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Return response
	return &CommunityResponse{
		ID:        community.ID.Hex(),
		Name:      community.Name,
		ImageHash: community.ImageHash,
		OwnerID:   community.OwnerID.Hex(),
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
		return nil, ErrInternalServerError.WithErr(err)
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
	err = a.database.CommunityService.UpdateCommunity(r.Context.Request.Context(), communityID, req.Name, req.ImageHash)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Get updated community
	updatedCommunity, err := a.database.CommunityService.GetCommunity(r.Context.Request.Context(), communityID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Return response
	return &CommunityResponse{
		ID:        updatedCommunity.ID.Hex(),
		Name:      updatedCommunity.Name,
		ImageHash: updatedCommunity.ImageHash,
		OwnerID:   updatedCommunity.OwnerID.Hex(),
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
		return nil, ErrInternalServerError.WithErr(err)
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
	page, err := r.Context.GetPage()
	if err != nil {
		return nil, ErrInvalidRequestBodyData.WithErr(err)
	}

	// Get community users
	users, err := a.database.CommunityService.GetCommunityUsers(r.Context.Request.Context(), communityID, page)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	// Convert to response format
	response := make([]CommunityUserResponse, len(users))
	for i, user := range users {
		// Find user's role in this community
		var role db.CommunityRole
		for _, comm := range user.Communities {
			if comm.ID == communityID {
				role = comm.Role
				break
			}
		}

		response[i] = CommunityUserResponse{
			ID:   user.ID.Hex(),
			Name: user.Name,
			Role: role,
		}
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

	// Create invitation
	invite, err := a.database.CommunityService.InviteUserToCommunity(r.Context.Request.Context(), communityID, userID, inviterID)
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
	}, nil
}

// leaveCommunityHandler handles POST /communities/{communityId}/leave
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

	// Get user ID
	userID, err := primitive.ObjectIDFromHex(r.UserID)
	if err != nil {
		return nil, ErrInvalidUserID.WithErr(err)
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

	// Get pending invites
	invites, err := a.database.CommunityService.GetUserPendingInvites(r.Context.Request.Context(), userID)
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
		}
	}

	return response, nil
}

// acceptInviteHandler handles POST /users/invites/{inviteId}/accept
func (a *API) acceptInviteHandler(r *Request) (interface{}, error) {
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

	// Accept invite
	err = a.database.CommunityService.AcceptInvite(r.Context.Request.Context(), inviteID, userID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	return nil, nil
}

// rejectInviteHandler handles POST /users/invites/{inviteId}/refuse
func (a *API) rejectInviteHandler(r *Request) (interface{}, error) {
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

	// Reject invite
	err = a.database.CommunityService.RejectInvite(r.Context.Request.Context(), inviteID, userID)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}

	return nil, nil
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

// These methods are now integrated into the original tool handlers in tools.go
// The functionality for handling communities with tools is now part of the
// original tool handlers, so these methods are no longer needed.
