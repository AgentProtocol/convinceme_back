package server

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/conversation"
	"github.com/neo/convinceme_backend/internal/database"
	"github.com/neo/convinceme_backend/internal/scoring"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDatabase for testing
type MockDatabaseForDebate struct {
	mock.Mock
}

// Ensure MockDatabaseForDebate implements database.DatabaseInterface
var _ database.DatabaseInterface = (*MockDatabaseForDebate)(nil)

func (m *MockDatabaseForDebate) CreateDebate(id, topic, status, agent1Name, agent2Name string) error {
	args := m.Called(id, topic, status, agent1Name, agent2Name)
	return args.Error(0)
}

func (m *MockDatabaseForDebate) UpdateDebateStatus(id, status string) error {
	args := m.Called(id, status)
	return args.Error(0)
}

func (m *MockDatabaseForDebate) UpdateDebateEnd(id, status, winner string) error {
	args := m.Called(id, status, winner)
	return args.Error(0)
}

func (m *MockDatabaseForDebate) GetDebate(id string) (*database.Debate, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.Debate), args.Error(1)
}

func (m *MockDatabaseForDebate) ListActiveDebates() ([]*database.Debate, error) {
	args := m.Called()
	return args.Get(0).([]*database.Debate), args.Error(1)
}

func (m *MockDatabaseForDebate) ListDebates(filter database.DebateFilter) ([]*database.Debate, int, error) {
	args := m.Called(filter)
	return args.Get(0).([]*database.Debate), args.Int(1), args.Error(2)
}

func (m *MockDatabaseForDebate) GetTopic(id int) (*database.Topic, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.Topic), args.Error(1)
}

func (m *MockDatabaseForDebate) GetTopics(filter database.TopicFilter) ([]*database.Topic, int, error) {
	args := m.Called(filter)
	return args.Get(0).([]*database.Topic), args.Int(1), args.Error(2)
}

func (m *MockDatabaseForDebate) SaveArgument(playerID, topic, content, side, debateID string) (int64, error) {
	args := m.Called(playerID, topic, content, side, debateID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockDatabaseForDebate) SaveScore(argumentID int64, debateID string, score *scoring.ArgumentScore) error {
	args := m.Called(argumentID, debateID, score)
	return args.Error(0)
}

func (m *MockDatabaseForDebate) GetAllArguments() ([]*database.Argument, error) {
	args := m.Called()
	return args.Get(0).([]*database.Argument), args.Error(1)
}

func (m *MockDatabaseForDebate) GetArgumentWithScore(id int64) (*database.Argument, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.Argument), args.Error(1)
}

// Stub implementations for other interface methods
func (m *MockDatabaseForDebate) Close() error {
	return nil
}

func (m *MockDatabaseForDebate) CreateUser(user *database.User, password string) error {
	return nil
}

func (m *MockDatabaseForDebate) GetUserByID(id string) (*database.User, error) {
	return nil, nil
}

func (m *MockDatabaseForDebate) GetUserByUsername(username string) (*database.User, error) {
	return nil, nil
}

func (m *MockDatabaseForDebate) GetUserByEmail(email string) (*database.User, error) {
	return nil, nil
}

func (m *MockDatabaseForDebate) UpdateUser(user *database.User) error {
	return nil
}

func (m *MockDatabaseForDebate) DeleteUser(id string) error {
	return nil
}

func (m *MockDatabaseForDebate) VerifyPassword(username, password string) (*database.User, error) {
	return nil, nil
}

func (m *MockDatabaseForDebate) UpdatePassword(userID, newPassword string) error {
	return nil
}

func (m *MockDatabaseForDebate) CreateRefreshToken(userID, token string, expiresAt time.Time) error {
	return nil
}

func (m *MockDatabaseForDebate) GetRefreshToken(token string) (*database.RefreshToken, error) {
	return nil, nil
}

func (m *MockDatabaseForDebate) DeleteRefreshToken(token string) error {
	return nil
}

func (m *MockDatabaseForDebate) DeleteUserRefreshTokens(userID string) error {
	return nil
}

func (m *MockDatabaseForDebate) CreatePasswordResetToken(email string) (string, error) {
	return "", nil
}

func (m *MockDatabaseForDebate) VerifyPasswordResetToken(token string) (*database.User, error) {
	return nil, nil
}

func (m *MockDatabaseForDebate) ResetPassword(token, newPassword string) error {
	return nil
}

func (m *MockDatabaseForDebate) VerifyEmail(token string) error {
	return nil
}

func (m *MockDatabaseForDebate) ResendVerificationEmail(email string) (string, error) {
	return "", nil
}

func (m *MockDatabaseForDebate) CreateInvitationCode(createdBy, email string, expiresIn time.Duration) (*database.InvitationCode, error) {
	return nil, nil
}

func (m *MockDatabaseForDebate) GetInvitationCode(code string) (*database.InvitationCode, error) {
	return nil, nil
}

func (m *MockDatabaseForDebate) ValidateInvitationCode(code string) (*database.InvitationCode, error) {
	return nil, nil
}

func (m *MockDatabaseForDebate) UseInvitationCode(code, usedBy string) error {
	return nil
}

func (m *MockDatabaseForDebate) GetInvitationsByUser(userID string) ([]*database.InvitationCode, error) {
	return nil, nil
}

func (m *MockDatabaseForDebate) DeleteInvitationCode(id int, userID string) error {
	return nil
}

func (m *MockDatabaseForDebate) CleanupExpiredInvitations() error {
	return nil
}

func (m *MockDatabaseForDebate) SaveFeedback(feedback *database.Feedback) error {
	return nil
}

func (m *MockDatabaseForDebate) GetFeedback(id int) (*database.Feedback, error) {
	return nil, nil
}

func (m *MockDatabaseForDebate) GetFeedbackByUser(userID string) ([]*database.Feedback, error) {
	return nil, nil
}

func (m *MockDatabaseForDebate) GetAllFeedback(filter database.FeedbackFilter) ([]*database.Feedback, int, error) {
	return nil, 0, nil
}

func (m *MockDatabaseForDebate) GetFeedbackStats() (map[string]interface{}, error) {
	return nil, nil
}

func (m *MockDatabaseForDebate) DeleteFeedback(id int) error {
	return nil
}

func (m *MockDatabaseForDebate) GetLeaderboard(debateID string, limit int) ([]*database.Argument, error) {
	args := m.Called(debateID, limit)
	return args.Get(0).([]*database.Argument), args.Error(1)
}

func (m *MockDatabaseForDebate) SubmitVote(userID string, argumentID int64, debateID string, voteType string) error {
	args := m.Called(userID, argumentID, debateID, voteType)
	return args.Error(0)
}

func (m *MockDatabaseForDebate) GetUserVoteCount(userID string, debateID string) (int, error) {
	args := m.Called(userID, debateID)
	return args.Int(0), args.Error(1)
}

func (m *MockDatabaseForDebate) HasUserPaidForComment(userID string, username string, debateID string) (bool, error) {
	args := m.Called(userID, username, debateID)
	return args.Bool(0), args.Error(1)
}

func (m *MockDatabaseForDebate) GetUserVoteForArgument(userID string, argumentID int64) (string, error) {
	args := m.Called(userID, argumentID)
	return args.String(0), args.Error(1)
}

func (m *MockDatabaseForDebate) CanUserVote(userID string, username string, argumentID int64, debateID string) (bool, string, error) {
	args := m.Called(userID, username, argumentID, debateID)
	return args.Bool(0), args.String(1), args.Error(2)
}

func (m *MockDatabaseForDebate) RunMigrations() error {
	args := m.Called()
	return args.Error(0)
}

// MockAgent for testing
type MockAgent struct {
	mock.Mock
	*agent.Agent
}

func (m *MockAgent) GetName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAgent) GenerateResponse(ctx context.Context, topic, prompt string) (string, error) {
	args := m.Called(ctx, topic, prompt)
	return args.String(0), args.Error(1)
}

// TestCreateDebate tests the CreateDebate function
func TestCreateDebate(t *testing.T) {
	// Create mock database
	mockDB := new(MockDatabaseForDebate)

	// Create real agents for the map and function call
	agent1 := &agent.Agent{}
	agent2 := &agent.Agent{}

	// Setup agents map
	agents := map[string]*agent.Agent{
		"Agent1": agent1,
		"Agent2": agent2,
	}

	// Setup debate manager
	debateManager := &DebateManager{
		db:      mockDB,
		agents:  agents,
		apiKey:  "test-api-key",
		debates: make(map[string]*conversation.DebateSession),
	}

	// Setup expectations
	mockDB.On("CreateDebate", mock.AnythingOfType("string"), "Test Topic", "waiting", mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(nil)

	// Call the function
	debateID, err := debateManager.CreateDebate("Test Topic", agent1, agent2, "test_user")

	// Assertions
	assert.NoError(t, err)
	assert.NotEmpty(t, debateID)
	mockDB.AssertExpectations(t)

	// Verify the debate was added to the map
	_, exists := debateManager.debates[debateID]
	assert.True(t, exists)
}

// TestGetDebate tests the GetDebate function
func TestGetDebate(t *testing.T) {
	// Create mock database
	mockDB := new(MockDatabaseForDebate)

	// Setup debate manager
	debateManager := &DebateManager{
		db:      mockDB,
		agents:  make(map[string]*agent.Agent),
		apiKey:  "test-api-key",
		debates: make(map[string]*conversation.DebateSession),
	}

	// Create a mock debate session
	testDebateID := "test-debate-id"
	mockSession := &conversation.DebateSession{}

	// Add the mock session to the debates map
	debateManager.debates[testDebateID] = mockSession

	// Test getting an existing debate
	session, exists := debateManager.GetDebate(testDebateID)
	assert.True(t, exists)
	assert.Equal(t, mockSession, session)

	// Test getting a non-existent debate
	session, exists = debateManager.GetDebate("non-existent-id")
	assert.False(t, exists)
	assert.Nil(t, session)
}

// TestRemoveDebate tests the RemoveDebate function
func TestRemoveDebate(t *testing.T) {
	// Create mock database
	mockDB := new(MockDatabaseForDebate)

	// Setup debate manager
	debateManager := &DebateManager{
		db:      mockDB,
		agents:  make(map[string]*agent.Agent),
		apiKey:  "test-api-key",
		debates: make(map[string]*conversation.DebateSession),
	}

	// Create a mock debate session
	testDebateID := "test-debate-id"
	mockSession := &conversation.DebateSession{}

	// Add the mock session to the debates map
	debateManager.debates[testDebateID] = mockSession

	// Verify the debate exists
	_, exists := debateManager.debates[testDebateID]
	assert.True(t, exists)

	// Remove the debate
	debateManager.RemoveDebate(testDebateID)

	// Verify the debate was removed
	_, exists = debateManager.debates[testDebateID]
	assert.False(t, exists)
}

// TestCleanupInactiveDebates tests the CleanupInactiveDebates function
func TestCleanupInactiveDebates(t *testing.T) {
	// Create mock database
	mockDB := new(MockDatabaseForDebate)

	// Setup debate manager
	debateManager := &DebateManager{
		db:      mockDB,
		agents:  make(map[string]*agent.Agent),
		apiKey:  "test-api-key",
		debates: make(map[string]*conversation.DebateSession),
	}

	// Create mock debate sessions with different statuses
	activeDebateID := "active-debate-id"
	finishedDebateID := "finished-debate-id"

	// Create mock sessions
	activeSession := &conversation.DebateSession{}
	finishedSession := &conversation.DebateSession{}

	// Set up the GetStatus method to return appropriate values
	// Since we can't mock methods on conversation.DebateSession directly,
	// we'll need to add real sessions with the appropriate status

	// Add sessions to the debates map
	debateManager.debates[activeDebateID] = activeSession
	debateManager.debates[finishedDebateID] = finishedSession

	// Set the status of the sessions
	activeSession.UpdateStatus("active")
	finishedSession.UpdateStatus("finished")

	// Call the function
	debateManager.CleanupInactiveDebates()

	// Verify active debate is still in the map
	_, exists := debateManager.debates[activeDebateID]
	assert.True(t, exists, "Active debate should not be removed")

	// Verify finished debate is removed
	_, exists = debateManager.debates[finishedDebateID]
	assert.False(t, exists, "Finished debate should be removed")
}

// TestNormalizeScore tests the NormalizeScore function
func TestNormalizeScore(t *testing.T) {
	// Setup debate manager
	debateManager := &DebateManager{
		db:      nil,
		agents:  make(map[string]*agent.Agent),
		apiKey:  "test-api-key",
		debates: make(map[string]*conversation.DebateSession),
	}

	// Test cases
	testCases := []struct {
		input    int
		expected float64
	}{
		{100, 5.0},  // 100/200*10 = 5.0
		{0, 0.0},    // 0/200*10 = 0.0
		{-50, 0.0},  // Negative should be clamped to 0
		{150, 7.5},  // 150/200*10 = 7.5
		{200, 10.0}, // 200/200*10 = 10.0
		{250, 10.0}, // >200 should be clamped to 10.0
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Input_%d", tc.input), func(t *testing.T) {
			result := debateManager.NormalizeScore(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}
