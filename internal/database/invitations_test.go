package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCreateAndGetInvitationCode(t *testing.T) {
	// Set up test database
	db, _, cleanup := setupUsersTestDB(t)
	defer cleanup()

	// Create a test user
	user := &User{
		ID:            "test-user-id",
		Username:      "testuser",
		Email:         "test@example.com",
		Role:          RoleUser,
		EmailVerified: true,
	}

	// Create the user
	err := db.CreateUser(user, "password123")
	assert.NoError(t, err)

	// Create an invitation code
	email := "invited@example.com"
	expiresIn := 7 * 24 * time.Hour
	invitation, err := db.CreateInvitationCode(user.ID, email, expiresIn)
	assert.NoError(t, err)
	assert.NotNil(t, invitation)
	assert.NotEmpty(t, invitation.Code)
	assert.Equal(t, user.ID, invitation.CreatedBy)
	assert.Equal(t, email, invitation.Email)
	assert.False(t, invitation.Used)
	assert.NotNil(t, invitation.ExpiresAt)
	assert.True(t, invitation.ExpiresAt.After(time.Now()))

	// Get the invitation code
	retrievedInvitation, err := db.GetInvitationCode(invitation.Code)
	assert.NoError(t, err)
	assert.Equal(t, invitation.ID, retrievedInvitation.ID)
	assert.Equal(t, invitation.Code, retrievedInvitation.Code)
	assert.Equal(t, invitation.CreatedBy, retrievedInvitation.CreatedBy)
	assert.Equal(t, invitation.Email, retrievedInvitation.Email)
	assert.Equal(t, invitation.Used, retrievedInvitation.Used)

	// Try to get a non-existent invitation code
	_, err = db.GetInvitationCode("non-existent-code")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestValidateInvitationCode(t *testing.T) {
	// Set up test database
	db, _, cleanup := setupUsersTestDB(t)
	defer cleanup()

	// Create a test user
	user := &User{
		ID:            "test-user-id",
		Username:      "testuser",
		Email:         "test@example.com",
		Role:          RoleUser,
		EmailVerified: true,
	}

	// Create the user
	err := db.CreateUser(user, "password123")
	assert.NoError(t, err)

	// Create an invitation code
	email := "invited@example.com"
	expiresIn := 7 * 24 * time.Hour
	invitation, err := db.CreateInvitationCode(user.ID, email, expiresIn)
	assert.NoError(t, err)

	// Validate the invitation code
	validatedInvitation, err := db.ValidateInvitationCode(invitation.Code)
	assert.NoError(t, err)
	assert.Equal(t, invitation.ID, validatedInvitation.ID)

	// Create an expired invitation code
	expiredInvitation, err := db.CreateInvitationCode(user.ID, "expired@example.com", -1*time.Hour)
	assert.NoError(t, err)

	// Try to validate the expired code
	_, err = db.ValidateInvitationCode(expiredInvitation.Code)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")

	// Try to validate a non-existent code
	_, err = db.ValidateInvitationCode("non-existent-code")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUseInvitationCode(t *testing.T) {
	// Set up test database
	db, _, cleanup := setupUsersTestDB(t)
	defer cleanup()

	// Create a test user (inviter)
	inviter := &User{
		ID:            "inviter-id",
		Username:      "inviter",
		Email:         "inviter@example.com",
		Role:          RoleUser,
		EmailVerified: true,
	}

	// Create the inviter
	err := db.CreateUser(inviter, "password123")
	assert.NoError(t, err)

	// Create an invitation code
	email := "invited@example.com"
	expiresIn := 7 * 24 * time.Hour
	invitation, err := db.CreateInvitationCode(inviter.ID, email, expiresIn)
	assert.NoError(t, err)

	// Create a test user (invitee)
	invitee := &User{
		ID:            "invitee-id",
		Username:      "invitee",
		Email:         email,
		Role:          RoleUser,
		EmailVerified: true,
	}

	// Create the invitee
	err = db.CreateUser(invitee, "password456")
	assert.NoError(t, err)

	// Use the invitation code
	err = db.UseInvitationCode(invitation.Code, invitee.ID)
	assert.NoError(t, err)

	// Get the invitation code again
	retrievedInvitation, err := db.GetInvitationCode(invitation.Code)
	assert.NoError(t, err)
	assert.True(t, retrievedInvitation.Used)
	assert.Equal(t, invitee.ID, retrievedInvitation.UsedBy)
	assert.NotNil(t, retrievedInvitation.UsedAt)

	// Try to use the code again
	err = db.UseInvitationCode(invitation.Code, "another-user-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already been used")

	// Try to use a non-existent code
	err = db.UseInvitationCode("non-existent-code", invitee.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestGetInvitationsByUser(t *testing.T) {
	// Set up test database
	db, _, cleanup := setupUsersTestDB(t)
	defer cleanup()

	// Create a test user
	user := &User{
		ID:            "test-user-id",
		Username:      "testuser",
		Email:         "test@example.com",
		Role:          RoleUser,
		EmailVerified: true,
	}

	// Create the user
	err := db.CreateUser(user, "password123")
	assert.NoError(t, err)

	// Create multiple invitation codes
	expiresIn := 7 * 24 * time.Hour
	invitation1, err := db.CreateInvitationCode(user.ID, "invite1@example.com", expiresIn)
	assert.NoError(t, err)
	invitation2, err := db.CreateInvitationCode(user.ID, "invite2@example.com", expiresIn)
	assert.NoError(t, err)

	// Create another user with their own invitations
	otherUser := &User{
		ID:            "other-user-id",
		Username:      "otheruser",
		Email:         "other@example.com",
		Role:          RoleUser,
		EmailVerified: true,
	}
	err = db.CreateUser(otherUser, "password789")
	assert.NoError(t, err)
	_, err = db.CreateInvitationCode(otherUser.ID, "otherinvite@example.com", expiresIn)
	assert.NoError(t, err)

	// Get invitations for the first user
	invitations, err := db.GetInvitationsByUser(user.ID)
	assert.NoError(t, err)
	assert.Len(t, invitations, 2)

	// Check that we got the correct invitations
	codes := []string{invitations[0].Code, invitations[1].Code}
	assert.Contains(t, codes, invitation1.Code)
	assert.Contains(t, codes, invitation2.Code)

	// Get invitations for a user with no invitations
	nonExistentUserID := "non-existent-user-id"
	invitations, err = db.GetInvitationsByUser(nonExistentUserID)
	assert.NoError(t, err)
	assert.Empty(t, invitations)
}

func TestDeleteInvitationCode(t *testing.T) {
	// Set up test database
	db, _, cleanup := setupUsersTestDB(t)
	defer cleanup()

	// Create a test user
	user := &User{
		ID:            "test-user-id",
		Username:      "testuser",
		Email:         "test@example.com",
		Role:          RoleUser,
		EmailVerified: true,
	}

	// Create the user
	err := db.CreateUser(user, "password123")
	assert.NoError(t, err)

	// Create an invitation code
	expiresIn := 7 * 24 * time.Hour
	invitation, err := db.CreateInvitationCode(user.ID, "invited@example.com", expiresIn)
	assert.NoError(t, err)

	// Create another user
	otherUser := &User{
		ID:            "other-user-id",
		Username:      "otheruser",
		Email:         "other@example.com",
		Role:          RoleUser,
		EmailVerified: true,
	}
	err = db.CreateUser(otherUser, "password789")
	assert.NoError(t, err)

	// Try to delete the invitation as the other user (should fail)
	err = db.DeleteInvitationCode(invitation.ID, otherUser.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not authorized")

	// Delete the invitation as the creator
	err = db.DeleteInvitationCode(invitation.ID, user.ID)
	assert.NoError(t, err)

	// Try to get the deleted invitation
	_, err = db.GetInvitationCode(invitation.Code)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Try to delete a non-existent invitation
	err = db.DeleteInvitationCode(9999, user.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCleanupExpiredInvitations(t *testing.T) {
	// Set up test database
	db, _, cleanup := setupUsersTestDB(t)
	defer cleanup()

	// Create a test user
	user := &User{
		ID:            "test-user-id",
		Username:      "testuser",
		Email:         "test@example.com",
		Role:          RoleUser,
		EmailVerified: true,
	}

	// Create the user
	err := db.CreateUser(user, "password123")
	assert.NoError(t, err)

	// Create an active invitation code
	activeInvitation, err := db.CreateInvitationCode(user.ID, "active@example.com", 7*24*time.Hour)
	assert.NoError(t, err)

	// Create an expired invitation code
	expiredInvitation, err := db.CreateInvitationCode(user.ID, "expired@example.com", -1*time.Hour)
	assert.NoError(t, err)

	// Create a used invitation code that is expired
	usedInvitation, err := db.CreateInvitationCode(user.ID, "used@example.com", -1*time.Hour)
	assert.NoError(t, err)
	err = db.UseInvitationCode(usedInvitation.Code, user.ID)
	assert.NoError(t, err)

	// Run cleanup
	err = db.CleanupExpiredInvitations()
	assert.NoError(t, err)

	// Check that the active invitation still exists
	_, err = db.GetInvitationCode(activeInvitation.Code)
	assert.NoError(t, err)

	// Check that the expired invitation was deleted
	_, err = db.GetInvitationCode(expiredInvitation.Code)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Check that the used invitation still exists (even though it's expired)
	retrievedUsedInvitation, err := db.GetInvitationCode(usedInvitation.Code)
	assert.NoError(t, err)
	assert.True(t, retrievedUsedInvitation.Used)
}
