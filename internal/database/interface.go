package database

import (
	"time"

	"github.com/neo/convinceme_backend/internal/scoring"
)

// DatabaseInterface defines the interface for database operations
type DatabaseInterface interface {
	Close() error

	// User management
	CreateUser(user *User, password string) error
	GetUserByID(id string) (*User, error)
	GetUserByUsername(username string) (*User, error)
	GetUserByEmail(email string) (*User, error)
	UpdateUser(user *User) error
	DeleteUser(id string) error
	VerifyPassword(username, password string) (*User, error)
	UpdatePassword(userID, newPassword string) error

	// Authentication
	CreateRefreshToken(userID, token string, expiresAt time.Time) error
	GetRefreshToken(token string) (*RefreshToken, error)
	DeleteRefreshToken(token string) error
	DeleteUserRefreshTokens(userID string) error
	CreatePasswordResetToken(email string) (string, error)
	VerifyPasswordResetToken(token string) (*User, error)
	ResetPassword(token, newPassword string) error
	VerifyEmail(token string) error
	ResendVerificationEmail(email string) (string, error)

	// Invitation codes
	CreateInvitationCode(createdBy, email string, expiresIn time.Duration) (*InvitationCode, error)
	GetInvitationCode(code string) (*InvitationCode, error)
	ValidateInvitationCode(code string) (*InvitationCode, error)
	UseInvitationCode(code, usedBy string) error
	GetInvitationsByUser(userID string) ([]*InvitationCode, error)
	DeleteInvitationCode(id int, userID string) error
	CleanupExpiredInvitations() error

	// Feedback
	SaveFeedback(feedback *Feedback) error
	GetFeedback(id int) (*Feedback, error)
	GetFeedbackByUser(userID string) ([]*Feedback, error)
	GetAllFeedback(filter FeedbackFilter) ([]*Feedback, int, error)
	GetFeedbackStats() (map[string]interface{}, error)
	DeleteFeedback(id int) error

	// Debates
	CreateDebate(id, topic, status, agent1Name, agent2Name string) error
	GetDebate(id string) (*Debate, error)
	ListActiveDebates() ([]*Debate, error)
	ListDebates(filter DebateFilter) ([]*Debate, int, error)
	UpdateDebateStatus(id, status string) error
	UpdateDebateEnd(id, status string, winner string) error

	// Topics
	GetTopic(id int) (*Topic, error)
	GetTopics(filter TopicFilter) ([]*Topic, int, error)

	// Arguments and scoring
	SaveArgument(playerID, topic, content, side, debateID string) (int64, error)
	SaveScore(argumentID int64, debateID string, score *scoring.ArgumentScore) error
	GetAllArguments() ([]*Argument, error)
	GetArgumentWithScore(id int64) (*Argument, error)
	GetLeaderboard(debateID string, limit int) ([]*Argument, error)

	// Voting system
	SubmitVote(userID string, argumentID int64, debateID string, voteType string) error
	GetUserVoteCount(userID string, debateID string) (int, error)
	HasUserPaidForComment(userID string, username string, debateID string) (bool, error)
	GetUserVoteForArgument(userID string, argumentID int64) (string, error)                              // Returns vote type or empty string
	CanUserVote(userID string, username string, argumentID int64, debateID string) (bool, string, error) // Returns canVote, reason, error

	// Migration runner
	RunMigrations() error
}

// Ensure Database implements DatabaseInterface
var _ DatabaseInterface = (*Database)(nil)
