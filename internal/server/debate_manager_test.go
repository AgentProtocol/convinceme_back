//go:build ignore

package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/neo/convinceme_backend/internal/agent"
	"github.com/neo/convinceme_backend/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDatabase for testing
type MockDatabaseForDebate struct {
	mock.Mock
	*database.Database // Embed the real database to satisfy the interface
}

func (m *MockDatabaseForDebate) CreateDebate(id, topic, status, agent1Name, agent2Name string) error {
	args := m.Called(id, topic, status, agent1Name, agent2Name)
	return args.Error(0)
}

func (m *MockDatabaseForDebate) UpdateDebateStatus(id, status string) error {
	args := m.Called(id, status)
	return args.Error(0)
}

func (m *MockDatabaseForDebate) EndDebate(id, winner string) error {
	args := m.Called(id, winner)
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

// MockAgent for testing
type MockAgentForDebate struct {
	mock.Mock
	*agent.Agent // Embed the real agent to satisfy the interface
}

func (m *MockAgentForDebate) GetName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAgentForDebate) GetVoice() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockAgentForDebate) GenerateResponse(ctx interface{}, topic, prompt string) (string, error) {
	args := m.Called(ctx, topic, prompt)
	return args.String(0), args.Error(1)
}

// TestCreateDebate tests the CreateDebate function
func TestCreateDebate(t *testing.T) {
	// Create mock database
	mockDB := new(MockDatabaseForDebate)

	// Create mock agents
	mockAgent1 := new(MockAgentForDebate)
	mockAgent1.On("GetName").Return("Agent1")
	mockAgent1.On("GetVoice").Return("voice1")

	mockAgent2 := new(MockAgentForDebate)
	mockAgent2.On("GetName").Return("Agent2")
	mockAgent2.On("GetVoice").Return("voice2")

	// Setup agents map
	agents := map[string]*agent.Agent{
		"Agent1": &agent.Agent{},
		"Agent2": &agent.Agent{},
	}

	// Setup debate manager
	debateManager := &DebateManager{
		db:      mockDB,
		agents:  agents,
		apiKey:  "test-api-key",
		debates: make(map[string]*DebateSession),
	}

	// Setup expectations
	mockDB.On("CreateDebate", mock.AnythingOfType("string"), "Test Topic", "waiting", "Agent1", "Agent2").Return(nil)

	// Call the function
	debateID, err := debateManager.CreateDebate("Test Topic", mockAgent1, mockAgent2, "test_user")

	// Assertions
	assert.NoError(t, err)
	assert.NotEmpty(t, debateID)
	mockDB.AssertExpectations(t)
	mockAgent1.AssertExpectations(t)
	mockAgent2.AssertExpectations(t)

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
		debates: make(map[string]*DebateSession),
	}

	// Add a test debate to the map
	testDebateID := "test-debate-id"
	testSession := &DebateSession{
		ID:     testDebateID,
		Config: DebateConfig{Topic: "Test Topic"},
	}
	debateManager.debates[testDebateID] = testSession

	// Test getting an existing debate
	session, exists := debateManager.GetDebate(testDebateID)
	assert.True(t, exists)
	assert.Equal(t, testSession, session)

	// Test getting a non-existent debate
	session, exists = debateManager.GetDebate("non-existent-id")
	assert.False(t, exists)
	assert.Nil(t, session)
}

// TestUpdateDebateStatus tests the UpdateDebateStatus function
func TestUpdateDebateStatus(t *testing.T) {
	// Create mock database
	mockDB := new(MockDatabaseForDebate)

	// Setup debate manager
	debateManager := &DebateManager{
		db:      mockDB,
		agents:  make(map[string]*agent.Agent),
		apiKey:  "test-api-key",
		debates: make(map[string]*DebateSession),
	}

	// Add a test debate to the map
	testDebateID := "test-debate-id"
	testSession := &DebateSession{
		ID:     testDebateID,
		Config: DebateConfig{Topic: "Test Topic"},
		status: "waiting",
	}
	debateManager.debates[testDebateID] = testSession

	// Setup expectations
	mockDB.On("UpdateDebateStatus", testDebateID, "active").Return(nil)

	// Call the function
	err := debateManager.UpdateDebateStatus(testDebateID, "active")

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, "active", testSession.status)
	mockDB.AssertExpectations(t)
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
		debates: make(map[string]*DebateSession),
	}

	// Add test debates to the map
	// Active debate
	activeDebateID := "active-debate-id"
	activeSession := &DebateSession{
		ID:           activeDebateID,
		Config:       DebateConfig{Topic: "Active Topic"},
		status:       "active",
		lastActivity: time.Now(),
	}
	debateManager.debates[activeDebateID] = activeSession

	// Inactive debate (old)
	inactiveDebateID := "inactive-debate-id"
	inactiveSession := &DebateSession{
		ID:           inactiveDebateID,
		Config:       DebateConfig{Topic: "Inactive Topic"},
		status:       "active",
		lastActivity: time.Now().Add(-30 * time.Minute), // 30 minutes old
	}
	debateManager.debates[inactiveDebateID] = inactiveSession

	// Finished debate
	finishedDebateID := "finished-debate-id"
	finishedSession := &DebateSession{
		ID:           finishedDebateID,
		Config:       DebateConfig{Topic: "Finished Topic"},
		status:       "finished",
		lastActivity: time.Now().Add(-10 * time.Minute), // 10 minutes old
	}
	debateManager.debates[finishedDebateID] = finishedSession

	// Call the function
	debateManager.CleanupInactiveDebates()

	// Assertions
	// Active debate should still be in the map
	_, exists := debateManager.debates[activeDebateID]
	assert.True(t, exists)

	// Inactive debate should be removed
	_, exists = debateManager.debates[inactiveDebateID]
	assert.False(t, exists)

	// Finished debate should be removed
	_, exists = debateManager.debates[finishedDebateID]
	assert.False(t, exists)
}

// TestNormalizeScore tests the NormalizeScore function
func TestNormalizeScore(t *testing.T) {
	// Setup debate manager
	debateManager := &DebateManager{
		db:      nil,
		agents:  make(map[string]*agent.Agent),
		apiKey:  "test-api-key",
		debates: make(map[string]*DebateSession),
	}

	// Test cases
	testCases := []struct {
		input    int
		expected int
	}{
		{100, 100},
		{0, 0},
		{-50, 0},
		{150, 100},
		{200, 100},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Input_%d", tc.input), func(t *testing.T) {
			result := debateManager.NormalizeScore(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestDebateSession tests the DebateSession methods
func TestDebateSession(t *testing.T) {
	// Create a test session
	session := &DebateSession{
		ID:     "test-session-id",
		Config: DebateConfig{Topic: "Test Topic"},
		status: "waiting",
		gameScore: GameScore{
			Agent1Score: 100,
			Agent2Score: 100,
		},
		history: make([]HistoryEntry, 0),
	}

	// Test GetStatus
	assert.Equal(t, "waiting", session.GetStatus())

	// Test UpdateStatus
	session.UpdateStatus("active")
	assert.Equal(t, "active", session.GetStatus())

	// Test GetGameScore
	score := session.GetGameScore()
	assert.Equal(t, 100, score.Agent1Score)
	assert.Equal(t, 100, score.Agent2Score)

	// Test UpdateGameScore
	updatedScore := session.UpdateGameScore(10, -10)
	assert.Equal(t, 110, updatedScore.Agent1Score)
	assert.Equal(t, 90, updatedScore.Agent2Score)

	// Test AddHistoryEntry
	session.AddHistoryEntry("player1", "Test message", true)
	assert.Equal(t, 1, len(session.history))
	assert.Equal(t, "player1", session.history[0].Speaker)
	assert.Equal(t, "Test message", session.history[0].Message)
	assert.True(t, session.history[0].IsPlayer)

	// Test AddClient and RemoveClient
	session.clients = make(map[string]interface{})
	session.AddClient(nil, "client1")
	assert.Equal(t, 1, len(session.clients))

	session.RemoveClient("client1")
	assert.Equal(t, 0, len(session.clients))
}
