package auth

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"crypto/elliptic"
	"encoding/json"
	"math/big"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// User represents a user in the system
type User struct {
	ID            string     `json:"id"`
	Username      string     `json:"username"`
	Email         string     `json:"email"`
	Role          string     `json:"role"`
	LastLogin     *time.Time `json:"last_login,omitempty"`
	AccountLocked bool       `json:"account_locked"`
	EmailVerified bool       `json:"email_verified"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Claims represents the JWT claims for internal tokens
type Claims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// PrivyClaims represents the JWT claims for Privy tokens
type PrivyClaims struct {
	Subject   string `json:"sub"` // Privy DID
	Issuer    string `json:"iss"` // Should be "privy.io"
	Audience  string `json:"aud"` // Your Privy App ID
	SessionID string `json:"sid"` // Session ID
	jwt.RegisteredClaims
}

// Config contains authentication configuration
type Config struct {
	JWTSecret                string
	TokenDuration            time.Duration
	RefreshTokenDuration     time.Duration
	RequireEmailVerification bool
	RequireInvitation        bool
	PrivyAppID               string // Privy App ID for validating external tokens
	PrivyVerificationKey     string // Privy verification key for ES256 tokens (optional)
}

// GetConfig returns the authentication configuration
func (a *Auth) GetConfig() Config {
	return a.config
}

// Auth handles authentication
type Auth struct {
	config Config
}

// New creates a new Auth instance
func New(config Config) *Auth {
	return &Auth{
		config: config,
	}
}

// TokenPair contains an access token and a refresh token
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// GenerateToken generates a JWT token for a user
func (a *Auth) GenerateToken(user User) (string, error) {
	// Create claims with user information
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.config.TokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "convinceme",
			Subject:   user.ID,
		},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with the secret key
	tokenString, err := token.SignedString([]byte(a.config.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %v", err)
	}

	return tokenString, nil
}

// GenerateTokenPair generates both an access token and a refresh token
func (a *Auth) GenerateTokenPair(user User) (*TokenPair, error) {
	// Generate access token
	accessToken, err := a.GenerateToken(user)
	if err != nil {
		return nil, err
	}

	// Generate refresh token (a secure random string)
	refreshToken := make([]byte, 32)
	_, err = rand.Read(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %v", err)
	}

	refreshTokenString := base64.URLEncoding.EncodeToString(refreshToken)

	// Calculate expiration time
	expiresAt := time.Now().Add(a.config.TokenDuration)

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshTokenString,
		ExpiresAt:    expiresAt,
	}, nil
}

// ValidateToken validates a JWT token (supports both internal HS256 and external ES256 Privy tokens)
func (a *Auth) ValidateToken(tokenString string) (*Claims, error) {
	// First, decode the token WITHOUT validation to check if it's from Privy
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		// If we can't even parse it, try as internal token
		return a.validateInternalToken(tokenString)
	}

	// Check if this is a Privy token by examining the issuer
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if issuer, exists := claims["iss"]; exists {
			if issStr, ok := issuer.(string); ok && issStr == "privy.io" {
				// This is a Privy token, validate it as ES256
				return a.validatePrivyToken(tokenString)
			}
		}
	}

	// Not a Privy token, validate as internal HS256 token
	return a.validateInternalToken(tokenString)
}

// validateInternalToken validates internal HS256 JWT tokens
func (a *Auth) validateInternalToken(tokenString string) (*Claims, error) {
	// Parse the token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return []byte(a.config.JWTSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}

	// Check if the token is valid
	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	// Extract claims
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("failed to extract claims")
	}

	return claims, nil
}

// validatePrivyToken validates Privy ES256 JWT tokens
func (a *Auth) validatePrivyToken(tokenString string) (*Claims, error) {
	if a.config.PrivyAppID == "" {
		return nil, errors.New("Privy App ID not configured")
	}

	// Parse the token WITHOUT validation to get the kid from header
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse Privy token: %v", err)
	}

	// Get the key ID from the token header
	kid, ok := token.Header["kid"].(string)
	if !ok {
		return nil, errors.New("missing kid in Privy token header")
	}

	// Get the public key for verification
	publicKey, err := a.getPrivyPublicKey(kid)
	if err != nil {
		return nil, fmt.Errorf("failed to get Privy public key: %v", err)
	}

	// Verify the token with the public key
	privyClaims := &PrivyClaims{}
	token, err = jwt.ParseWithClaims(tokenString, privyClaims, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to verify Privy token: %v", err)
	}

	if !token.Valid {
		return nil, errors.New("invalid Privy token")
	}

	// Validate issuer and audience
	if privyClaims.Issuer != "privy.io" {
		return nil, fmt.Errorf("invalid issuer: %s", privyClaims.Issuer)
	}

	if privyClaims.Audience != a.config.PrivyAppID {
		return nil, fmt.Errorf("invalid audience: %s", privyClaims.Audience)
	}

	// Convert Privy claims to internal Claims format
	internalClaims := &Claims{
		UserID:   privyClaims.Subject, // Use Privy DID as user ID
		Username: privyClaims.Subject, // Default to DID, can be enhanced later
		Email:    "",                  // Not available in Privy token by default
		Role:     "user",              // Default role for Privy users
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: privyClaims.ExpiresAt,
			IssuedAt:  privyClaims.IssuedAt,
			NotBefore: privyClaims.NotBefore,
			Issuer:    privyClaims.Issuer,
			Subject:   privyClaims.Subject,
			Audience:  jwt.ClaimStrings{privyClaims.Audience},
		},
	}

	return internalClaims, nil
}

// JWKSet represents a JSON Web Key Set
type JWKSet struct {
	Keys []JWK `json:"keys"`
}

// JWK represents a JSON Web Key
type JWK struct {
	KeyType   string `json:"kty"`
	KeyID     string `json:"kid"`
	Use       string `json:"use"`
	Algorithm string `json:"alg"`
	Curve     string `json:"crv"`
	X         string `json:"x"`
	Y         string `json:"y"`
}

// getPrivyPublicKey retrieves the public key for Privy token verification
func (a *Auth) getPrivyPublicKey(kid string) (*ecdsa.PublicKey, error) {
	// If a verification key is provided in config, use it
	if a.config.PrivyVerificationKey != "" {
		return a.parsePrivyPublicKey(a.config.PrivyVerificationKey)
	}

	// Use the app-specific JWKS endpoint
	jwksURL := fmt.Sprintf("https://auth.privy.io/api/v1/apps/%s/jwks.json", a.config.PrivyAppID)

	resp, err := http.Get(jwksURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read JWKS response: %v", err)
	}

	var jwks JWKSet
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("failed to parse JWKS: %v", err)
	}

	// Find the key with matching kid
	for _, key := range jwks.Keys {
		if key.KeyID == kid && key.KeyType == "EC" && key.Algorithm == "ES256" {
			return a.jwkToPublicKey(key)
		}
	}

	return nil, fmt.Errorf("no matching key found for kid: %s", kid)
}

// jwkToPublicKey converts a JWK to an ECDSA public key
func (a *Auth) jwkToPublicKey(jwk JWK) (*ecdsa.PublicKey, error) {
	// Decode the x and y coordinates
	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, fmt.Errorf("failed to decode x coordinate: %v", err)
	}

	yBytes, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		return nil, fmt.Errorf("failed to decode y coordinate: %v", err)
	}

	// Create the public key
	pubKey := &ecdsa.PublicKey{}

	// Set the curve based on the JWK curve parameter
	switch jwk.Curve {
	case "P-256":
		pubKey.Curve = elliptic.P256()
	case "P-384":
		pubKey.Curve = elliptic.P384()
	case "P-521":
		pubKey.Curve = elliptic.P521()
	default:
		return nil, fmt.Errorf("unsupported curve: %s", jwk.Curve)
	}

	// Set the coordinates
	pubKey.X = new(big.Int).SetBytes(xBytes)
	pubKey.Y = new(big.Int).SetBytes(yBytes)

	return pubKey, nil
}

// parsePrivyPublicKey parses a PEM-encoded public key string
func (a *Auth) parsePrivyPublicKey(keyStr string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(keyStr))
	if block == nil {
		return nil, errors.New("failed to parse PEM block")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %v", err)
	}

	ecdsaKey, ok := pubKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("not an ECDSA public key")
	}

	return ecdsaKey, nil
}

// GenerateRandomKey generates a random key for JWT signing
func GenerateRandomKey(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// AuthMiddleware returns a middleware that checks for a valid JWT token
func (a *Auth) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		// Check if the Authorization header has the correct format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header format must be Bearer {token}"})
			c.Abort()
			return
		}

		// Validate the token
		tokenString := parts[1]
		claims, err := a.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("Invalid token: %v", err)})
			c.Abort()
			return
		}

		// Set user information in the context
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// OptionalAuthMiddleware returns a middleware that checks for a valid JWT token but doesn't require it
func (a *Auth) OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// No token provided, continue without user info
			c.Next()
			return
		}

		// Check if the Authorization header has the correct format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			// Invalid format, continue without user info
			c.Next()
			return
		}

		// Validate the token
		tokenString := parts[1]
		claims, err := a.ValidateToken(tokenString)
		if err != nil {
			// Invalid token, continue without user info
			c.Next()
			return
		}

		// Set user information in the context
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// GetUserID gets the user ID from the context
func GetUserID(c *gin.Context) (string, bool) {
	userID, exists := c.Get("userID")
	if !exists {
		return "", false
	}
	return userID.(string), true
}

// GetUsername gets the username from the context
func GetUsername(c *gin.Context) (string, bool) {
	username, exists := c.Get("username")
	if !exists {
		return "", false
	}
	return username.(string), true
}

// GetUserRole gets the user role from the context
func GetUserRole(c *gin.Context) (string, bool) {
	role, exists := c.Get("role")
	if !exists {
		return "", false
	}
	return role.(string), true
}

// GetUserEmail gets the user email from the context
func GetUserEmail(c *gin.Context) (string, bool) {
	email, exists := c.Get("email")
	if !exists {
		return "", false
	}
	return email.(string), true
}

// RequireRole returns a middleware that requires a specific role
func (a *Auth) RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the user role from the context (set by AuthMiddleware)
		userRole, exists := GetUserRole(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization required"})
			c.Abort()
			return
		}

		// Check if the user has the required role
		if userRole != role && userRole != "admin" { // Admins can access everything
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}
