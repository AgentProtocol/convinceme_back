package server

import (
	"errors"
	"time"

	"github.com/neo/convinceme_backend/internal/database"
	"github.com/neo/convinceme_backend/internal/scoring"
)

// timePtr returns a pointer to the given time.Time
func timePtr(t time.Time) *time.Time {
	return &t
}

// TestMockDB is a mock implementation of the database for testing
type TestMockDB struct {
	// Add fields if needed for tracking state
}

// Ensure TestMockDB implements database.DatabaseInterface
var _ database.DatabaseInterface = (*TestMockDB)(nil)

// Close closes the database connection
func (m *TestMockDB) Close() error {
	return nil
}

// CreateDebate adds a new debate session to the database
func (m *TestMockDB) CreateDebate(id, topic, status, agent1Name, agent2Name string) error {
	// For testing, just return nil (success)
	return nil
}

// CreateUser creates a new user
func (m *TestMockDB) CreateUser(user *database.User, password string) error {
	if user.Username == "duplicate" {
		return errors.New("username already exists")
	}
	if user.Email == "same@example.com" && user.Username != "email1" {
		return errors.New("email already exists")
	}
	return nil
}

// GetUserByID gets a user by ID
func (m *TestMockDB) GetUserByID(id string) (*database.User, error) {
	if id == "test-user-id" {
		return &database.User{
			ID:            id,
			Username:      "testuser",
			Email:         "test@example.com",
			Role:          database.RoleUser,
			EmailVerified: true,
		}, nil
	}
	return nil, errors.New("user not found")
}

// GetUserByUsername gets a user by username
func (m *TestMockDB) GetUserByUsername(username string) (*database.User, error) {
	if username == "logintest" {
		return &database.User{
			ID:            "test-user-id",
			Username:      username,
			Email:         "login@example.com",
			Role:          database.RoleUser,
			EmailVerified: true,
			PasswordHash:  "Password123!",
		}, nil
	}
	if username == "forgottest" {
		return &database.User{
			ID:            "test-user-id",
			Username:      username,
			Email:         "forgot@example.com",
			Role:          database.RoleUser,
			EmailVerified: true,
		}, nil
	}
	if username == "resettest" {
		return &database.User{
			ID:            "test-user-id",
			Username:      username,
			Email:         "reset@example.com",
			Role:          database.RoleUser,
			EmailVerified: true,
		}, nil
	}
	return nil, errors.New("user not found")
}

// GetUserByEmail gets a user by email
func (m *TestMockDB) GetUserByEmail(email string) (*database.User, error) {
	if email == "forgot@example.com" {
		return &database.User{
			ID:            "test-user-id",
			Username:      "forgottest",
			Email:         email,
			Role:          database.RoleUser,
			EmailVerified: true,
		}, nil
	}
	if email == "reset@example.com" {
		return &database.User{
			ID:            "test-user-id",
			Username:      "resettest",
			Email:         email,
			Role:          database.RoleUser,
			EmailVerified: true,
		}, nil
	}
	return nil, errors.New("user not found")
}

// UpdateUser updates a user
func (m *TestMockDB) UpdateUser(user *database.User) error {
	return nil
}

// DeleteUser deletes a user
func (m *TestMockDB) DeleteUser(id string) error {
	return nil
}

// VerifyPassword verifies a user's password
func (m *TestMockDB) VerifyPassword(username, password string) (*database.User, error) {
	if username == "logintest" && password == "Password123!" {
		return &database.User{
			ID:            "test-user-id",
			Username:      username,
			Email:         "login@example.com",
			Role:          database.RoleUser,
			EmailVerified: true,
		}, nil
	}
	return nil, errors.New("invalid credentials")
}

// UpdatePassword updates a user's password
func (m *TestMockDB) UpdatePassword(userID, newPassword string) error {
	return nil
}

// CreateRefreshToken creates a refresh token
func (m *TestMockDB) CreateRefreshToken(userID, token string, expiresAt time.Time) error {
	return nil
}

// GetRefreshToken gets a refresh token
func (m *TestMockDB) GetRefreshToken(token string) (*database.RefreshToken, error) {
	if token == "valid-refresh-token" {
		return &database.RefreshToken{
			Token:     token,
			UserID:    "test-user-id",
			ExpiresAt: time.Now().Add(time.Hour),
		}, nil
	}
	return nil, errors.New("refresh token not found")
}

// DeleteRefreshToken deletes a refresh token
func (m *TestMockDB) DeleteRefreshToken(token string) error {
	return nil
}

// DeleteUserRefreshTokens deletes all refresh tokens for a user
func (m *TestMockDB) DeleteUserRefreshTokens(userID string) error {
	return nil
}

// CreatePasswordResetToken creates a password reset token
func (m *TestMockDB) CreatePasswordResetToken(email string) (string, error) {
	if email == "forgot@example.com" || email == "reset@example.com" {
		return "valid-reset-token", nil
	}
	return "", errors.New("user not found")
}

// VerifyPasswordResetToken verifies a password reset token
func (m *TestMockDB) VerifyPasswordResetToken(token string) (*database.User, error) {
	if token == "valid-reset-token" {
		return &database.User{
			ID:            "test-user-id",
			Username:      "resettest",
			Email:         "reset@example.com",
			Role:          database.RoleUser,
			EmailVerified: true,
		}, nil
	}
	return nil, errors.New("invalid reset token")
}

// ResetPassword resets a user's password using a reset token
func (m *TestMockDB) ResetPassword(token, newPassword string) error {
	if token == "valid-reset-token" {
		return nil
	}
	return errors.New("invalid reset token")
}

// VerifyEmail verifies a user's email
func (m *TestMockDB) VerifyEmail(token string) error {
	if token == "valid-verification-token" {
		return nil
	}
	return errors.New("invalid verification token")
}

// ResendVerificationEmail resends a verification email
func (m *TestMockDB) ResendVerificationEmail(email string) (string, error) {
	return "new-verification-token", nil
}

// CreateInvitationCode creates an invitation code
func (m *TestMockDB) CreateInvitationCode(createdBy, email string, expiresIn time.Duration) (*database.InvitationCode, error) {
	return &database.InvitationCode{
		ID:        1,
		Code:      "test-invitation-code",
		CreatedBy: createdBy,
		Email:     email,
		Used:      false,
		ExpiresAt: timePtr(time.Now().Add(expiresIn)),
		CreatedAt: time.Now(),
	}, nil
}

// GetInvitationCode gets an invitation code
func (m *TestMockDB) GetInvitationCode(code string) (*database.InvitationCode, error) {
	if code == "test-invitation-code" {
		return &database.InvitationCode{
			ID:        1,
			Code:      code,
			CreatedBy: "test-user-id",
			Email:     "invited@example.com",
			Used:      false,
			ExpiresAt: timePtr(time.Now().Add(time.Hour)),
			CreatedAt: time.Now(),
		}, nil
	}
	if code == "expired-invitation-code" {
		return &database.InvitationCode{
			ID:        2,
			Code:      code,
			CreatedBy: "test-user-id",
			Email:     "expired@example.com",
			Used:      false,
			ExpiresAt: timePtr(time.Now().Add(-time.Hour)),
			CreatedAt: time.Now().Add(-time.Hour * 2),
		}, nil
	}
	return nil, errors.New("invitation code not found")
}

// ValidateInvitationCode validates an invitation code
func (m *TestMockDB) ValidateInvitationCode(code string) (*database.InvitationCode, error) {
	if code == "test-invitation-code" {
		return &database.InvitationCode{
			ID:        1,
			Code:      code,
			CreatedBy: "test-user-id",
			Email:     "invited@example.com",
			Used:      false,
			ExpiresAt: timePtr(time.Now().Add(time.Hour)),
			CreatedAt: time.Now(),
		}, nil
	}
	if code == "expired-invitation-code" {
		return nil, errors.New("invitation code expired")
	}
	return nil, errors.New("invitation code not found")
}

// UseInvitationCode marks an invitation code as used
func (m *TestMockDB) UseInvitationCode(code, usedBy string) error {
	if code == "test-invitation-code" {
		return nil
	}
	return errors.New("invitation code not found")
}

// GetInvitationsByUser gets all invitation codes created by a user
func (m *TestMockDB) GetInvitationsByUser(userID string) ([]*database.InvitationCode, error) {
	if userID == "inviter-id" {
		return []*database.InvitationCode{
			{
				ID:        1,
				Code:      "test-invitation-code-1",
				CreatedBy: userID,
				Email:     "invite1@example.com",
				Used:      false,
				ExpiresAt: timePtr(time.Now().Add(time.Hour)),
				CreatedAt: time.Now(),
			},
			{
				ID:        2,
				Code:      "test-invitation-code-2",
				CreatedBy: userID,
				Email:     "invite2@example.com",
				Used:      false,
				ExpiresAt: timePtr(time.Now().Add(time.Hour)),
				CreatedAt: time.Now(),
			},
		}, nil
	}
	return []*database.InvitationCode{}, nil
}

// DeleteInvitationCode deletes an invitation code
func (m *TestMockDB) DeleteInvitationCode(id int, userID string) error {
	if id == 1 && userID == "user1-id" {
		return nil
	}
	if id != 1 {
		return errors.New("invitation code not found")
	}
	return errors.New("not authorized to delete this invitation code")
}

// CleanupExpiredInvitations cleans up expired invitation codes
func (m *TestMockDB) CleanupExpiredInvitations() error {
	return nil
}

// GetDebate gets a debate by ID
func (m *TestMockDB) GetDebate(id string) (*database.Debate, error) {
	return &database.Debate{
		ID:         id,
		Topic:      "Test Topic",
		Status:     "active",
		Agent1Name: "Agent 1",
		Agent2Name: "Agent 2",
		CreatedAt:  time.Now(),
	}, nil
}

// ListActiveDebates lists all active debates
func (m *TestMockDB) ListActiveDebates() ([]*database.Debate, error) {
	return []*database.Debate{
		{
			ID:         "debate-1",
			Topic:      "Test Topic 1",
			Status:     "active",
			Agent1Name: "Agent 1",
			Agent2Name: "Agent 2",
			CreatedAt:  time.Now(),
		},
		{
			ID:         "debate-2",
			Topic:      "Test Topic 2",
			Status:     "waiting",
			Agent1Name: "Agent 3",
			Agent2Name: "Agent 4",
			CreatedAt:  time.Now(),
		},
	}, nil
}

// ListDebates lists debates with pagination and filtering
func (m *TestMockDB) ListDebates(filter database.DebateFilter) ([]*database.Debate, int, error) {
	return []*database.Debate{
		{
			ID:         "debate-1",
			Topic:      "Test Topic 1",
			Status:     "active",
			Agent1Name: "Agent 1",
			Agent2Name: "Agent 2",
			CreatedAt:  time.Now(),
		},
	}, 1, nil
}

// UpdateDebateStatus updates a debate's status
func (m *TestMockDB) UpdateDebateStatus(id, status string) error {
	return nil
}

// UpdateDebateEnd updates a debate's end status and winner
func (m *TestMockDB) UpdateDebateEnd(id, status string, winner string) error {
	return nil
}

// GetTopic gets a topic by ID
func (m *TestMockDB) GetTopic(id int) (*database.Topic, error) {
	return &database.Topic{
		ID:          id,
		Title:       "Test Topic",
		Description: "Test Description",
		Agent1Name:  "Agent 1",
		Agent1Role:  "Role 1",
		Agent2Name:  "Agent 2",
		Agent2Role:  "Role 2",
		Category:    "Test Category",
		CreatedAt:   time.Now(),
	}, nil
}

// GetTopics gets topics with pagination and filtering
func (m *TestMockDB) GetTopics(filter database.TopicFilter) ([]*database.Topic, int, error) {
	return []*database.Topic{
		{
			ID:          1,
			Title:       "Test Topic 1",
			Description: "Test Description 1",
			Agent1Name:  "Agent 1",
			Agent1Role:  "Role 1",
			Agent2Name:  "Agent 2",
			Agent2Role:  "Role 2",
			Category:    "Test Category",
			CreatedAt:   time.Now(),
		},
	}, 1, nil
}

// SaveArgument saves an argument
func (m *TestMockDB) SaveArgument(playerID, topic, content, side, debateID string) (int64, error) {
	return 1, nil
}

// SaveScore saves a score for an argument
func (m *TestMockDB) SaveScore(argumentID int64, debateID string, score *scoring.ArgumentScore) error {
	return nil
}

// GetAllArguments gets all arguments
func (m *TestMockDB) GetAllArguments() ([]*database.Argument, error) {
	debateID := "debate-1"
	return []*database.Argument{
		{
			ID:        1,
			PlayerID:  "player-1",
			Topic:     "Test Topic",
			Content:   "Test Content",
			Side:      "pro",
			DebateID:  &debateID,
			CreatedAt: "2023-01-01T12:00:00Z",
		},
	}, nil
}

// GetArgumentWithScore gets an argument with its score
func (m *TestMockDB) GetArgumentWithScore(id int64) (*database.Argument, error) {
	debateID := "debate-1"
	return &database.Argument{
		ID:        id,
		PlayerID:  "player-1",
		Topic:     "Test Topic",
		Content:   "Test Content",
		Side:      "pro",
		DebateID:  &debateID,
		CreatedAt: "2023-01-01T12:00:00Z",
		Score:     &scoring.ArgumentScore{Average: 0.8},
	}, nil
}

// GetLeaderboard gets the top-scoring arguments for a specific debate
func (m *TestMockDB) GetLeaderboard(debateID string, limit int) ([]*database.Argument, error) {
	return []*database.Argument{
		{
			ID:        1,
			PlayerID:  "player-1",
			Topic:     "Test Topic",
			Content:   "Best argument ever!",
			Side:      "pro",
			DebateID:  &debateID,
			CreatedAt: "2023-01-01T12:00:00Z",
			Score:     &scoring.ArgumentScore{Average: 9.5, Strength: 10, Relevance: 9, Logic: 10, Truth: 9, Humor: 9},
		},
		{
			ID:        2,
			PlayerID:  "player-2",
			Topic:     "Test Topic",
			Content:   "Great argument with solid logic",
			Side:      "con",
			DebateID:  &debateID,
			CreatedAt: "2023-01-01T12:05:00Z",
			Score:     &scoring.ArgumentScore{Average: 8.2, Strength: 8, Relevance: 8, Logic: 9, Truth: 8, Humor: 8},
		},
	}, nil
}

// SaveFeedback saves feedback to the database
func (m *TestMockDB) SaveFeedback(feedback *database.Feedback) error {
	feedback.ID = 1
	return nil
}

// GetFeedback gets feedback by ID
func (m *TestMockDB) GetFeedback(id int) (*database.Feedback, error) {
	userID := "test-user-id"
	rating := 5
	return &database.Feedback{
		ID:         id,
		UserID:     &userID,
		Type:       database.FeedbackTypeAuth,
		Message:    "Test feedback",
		Rating:     &rating,
		Path:       "/test",
		Browser:    "Chrome",
		Device:     "Desktop",
		ScreenSize: "1920x1080",
		CreatedAt:  time.Now(),
	}, nil
}

// GetFeedbackByUser gets all feedback submitted by a user
func (m *TestMockDB) GetFeedbackByUser(userID string) ([]*database.Feedback, error) {
	rating := 5
	return []*database.Feedback{
		{
			ID:         1,
			UserID:     &userID,
			Type:       database.FeedbackTypeAuth,
			Message:    "Test feedback 1",
			Rating:     &rating,
			Path:       "/test1",
			Browser:    "Chrome",
			Device:     "Desktop",
			ScreenSize: "1920x1080",
			CreatedAt:  time.Now(),
		},
		{
			ID:         2,
			UserID:     &userID,
			Type:       database.FeedbackTypeBug,
			Message:    "Test feedback 2",
			Rating:     &rating,
			Path:       "/test2",
			Browser:    "Firefox",
			Device:     "Mobile",
			ScreenSize: "375x812",
			CreatedAt:  time.Now(),
		},
	}, nil
}

// GetAllFeedback gets all feedback with filtering and pagination
func (m *TestMockDB) GetAllFeedback(filter database.FeedbackFilter) ([]*database.Feedback, int, error) {
	userID := "test-user-id"
	rating := 5
	return []*database.Feedback{
		{
			ID:         1,
			UserID:     &userID,
			Type:       database.FeedbackTypeAuth,
			Message:    "Test feedback 1",
			Rating:     &rating,
			Path:       "/test1",
			Browser:    "Chrome",
			Device:     "Desktop",
			ScreenSize: "1920x1080",
			CreatedAt:  time.Now(),
		},
	}, 1, nil
}

// GetFeedbackStats gets statistics about feedback
func (m *TestMockDB) GetFeedbackStats() (map[string]interface{}, error) {
	return map[string]interface{}{
		"total_count": 10,
		"count_by_type": map[string]int{
			"auth":    3,
			"bug":     5,
			"feature": 2,
		},
		"average_rating": 4.2,
		"rating_distribution": map[string]int{
			"1": 1,
			"2": 1,
			"3": 2,
			"4": 3,
			"5": 3,
		},
		"feedback_over_time": map[string]int{
			"2023-01-01": 2,
			"2023-01-02": 3,
			"2023-01-03": 5,
		},
	}, nil
}

// DeleteFeedback deletes feedback by ID
func (m *TestMockDB) DeleteFeedback(id int) error {
	return nil
}
