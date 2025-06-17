package database

import (
	"database/sql"
	"fmt"
	"time"
)

// InvitationCode represents an invitation code for the closed alpha
type InvitationCode struct {
	ID        int        `json:"id"`
	Code      string     `json:"code"`
	CreatedBy string     `json:"created_by,omitempty"`
	Email     string     `json:"email,omitempty"`
	Used      bool       `json:"used"`
	UsedBy    string     `json:"used_by,omitempty"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// CreateInvitationCode creates a new invitation code
func (d *Database) CreateInvitationCode(createdBy string, email string, expiresIn time.Duration) (*InvitationCode, error) {
	// Generate a unique code
	code := generateRandomToken(8)
	
	// Calculate expiration time
	expiresAt := time.Now().Add(expiresIn)
	
	// Insert the invitation code
	query := `INSERT INTO invitation_codes (code, created_by, email, expires_at) VALUES (?, ?, ?, ?)`
	result, err := d.db.Exec(query, code, createdBy, email, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create invitation code: %v", err)
	}
	
	// Get the ID of the inserted code
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get invitation code ID: %v", err)
	}
	
	// Return the invitation code
	return &InvitationCode{
		ID:        int(id),
		Code:      code,
		CreatedBy: createdBy,
		Email:     email,
		Used:      false,
		ExpiresAt: &expiresAt,
		CreatedAt: time.Now(),
	}, nil
}

// GetInvitationCode gets an invitation code by code
func (d *Database) GetInvitationCode(code string) (*InvitationCode, error) {
	query := `SELECT id, code, created_by, email, used, used_by, used_at, expires_at, created_at 
			  FROM invitation_codes WHERE code = ?`
	
	var invitation InvitationCode
	var createdBy, email, usedBy sql.NullString
	var usedAt, expiresAt sql.NullTime
	
	err := d.db.QueryRow(query, code).Scan(
		&invitation.ID,
		&invitation.Code,
		&createdBy,
		&email,
		&invitation.Used,
		&usedBy,
		&usedAt,
		&expiresAt,
		&invitation.CreatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invitation code not found")
		}
		return nil, fmt.Errorf("failed to get invitation code: %v", err)
	}
	
	// Handle nullable fields
	if createdBy.Valid {
		invitation.CreatedBy = createdBy.String
	}
	
	if email.Valid {
		invitation.Email = email.String
	}
	
	if usedBy.Valid {
		invitation.UsedBy = usedBy.String
	}
	
	if usedAt.Valid {
		invitation.UsedAt = &usedAt.Time
	}
	
	if expiresAt.Valid {
		invitation.ExpiresAt = &expiresAt.Time
	}
	
	return &invitation, nil
}

// ValidateInvitationCode validates an invitation code
func (d *Database) ValidateInvitationCode(code string) (*InvitationCode, error) {
	// Get the invitation code
	invitation, err := d.GetInvitationCode(code)
	if err != nil {
		return nil, err
	}
	
	// Check if the code is already used
	if invitation.Used {
		return nil, fmt.Errorf("invitation code has already been used")
	}
	
	// Check if the code has expired
	if invitation.ExpiresAt != nil && time.Now().After(*invitation.ExpiresAt) {
		return nil, fmt.Errorf("invitation code has expired")
	}
	
	return invitation, nil
}

// UseInvitationCode marks an invitation code as used
func (d *Database) UseInvitationCode(code string, userID string) error {
	// Validate the code first
	invitation, err := d.ValidateInvitationCode(code)
	if err != nil {
		return err
	}
	
	// Mark the code as used
	now := time.Now()
	query := `UPDATE invitation_codes SET used = TRUE, used_by = ?, used_at = ? WHERE id = ?`
	_, err = d.db.Exec(query, userID, now, invitation.ID)
	if err != nil {
		return fmt.Errorf("failed to mark invitation code as used: %v", err)
	}
	
	return nil
}

// GetInvitationsByUser gets all invitation codes created by a user
func (d *Database) GetInvitationsByUser(userID string) ([]*InvitationCode, error) {
	query := `SELECT id, code, created_by, email, used, used_by, used_at, expires_at, created_at 
			  FROM invitation_codes WHERE created_by = ? ORDER BY created_at DESC`
	
	rows, err := d.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get invitations: %v", err)
	}
	defer rows.Close()
	
	var invitations []*InvitationCode
	
	for rows.Next() {
		var invitation InvitationCode
		var createdBy, email, usedBy sql.NullString
		var usedAt, expiresAt sql.NullTime
		
		err := rows.Scan(
			&invitation.ID,
			&invitation.Code,
			&createdBy,
			&email,
			&invitation.Used,
			&usedBy,
			&usedAt,
			&expiresAt,
			&invitation.CreatedAt,
		)
		
		if err != nil {
			return nil, fmt.Errorf("failed to scan invitation: %v", err)
		}
		
		// Handle nullable fields
		if createdBy.Valid {
			invitation.CreatedBy = createdBy.String
		}
		
		if email.Valid {
			invitation.Email = email.String
		}
		
		if usedBy.Valid {
			invitation.UsedBy = usedBy.String
		}
		
		if usedAt.Valid {
			invitation.UsedAt = &usedAt.Time
		}
		
		if expiresAt.Valid {
			invitation.ExpiresAt = &expiresAt.Time
		}
		
		invitations = append(invitations, &invitation)
	}
	
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating invitations: %v", err)
	}
	
	return invitations, nil
}

// DeleteInvitationCode deletes an invitation code
func (d *Database) DeleteInvitationCode(id int, userID string) error {
	// Check if the user is the creator of the code
	query := `SELECT created_by FROM invitation_codes WHERE id = ?`
	var createdBy sql.NullString
	
	err := d.db.QueryRow(query, id).Scan(&createdBy)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("invitation code not found")
		}
		return fmt.Errorf("failed to get invitation code: %v", err)
	}
	
	// Only allow the creator to delete the code
	if !createdBy.Valid || createdBy.String != userID {
		return fmt.Errorf("you are not authorized to delete this invitation code")
	}
	
	// Delete the code
	query = `DELETE FROM invitation_codes WHERE id = ?`
	_, err = d.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete invitation code: %v", err)
	}
	
	return nil
}

// CleanupExpiredInvitations removes all expired and unused invitation codes
func (d *Database) CleanupExpiredInvitations() error {
	query := `DELETE FROM invitation_codes WHERE used = FALSE AND expires_at < ?`
	_, err := d.db.Exec(query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to cleanup expired invitations: %v", err)
	}
	
	return nil
}
