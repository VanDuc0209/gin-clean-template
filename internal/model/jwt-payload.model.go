package model

import "time"

// JWTPayload represents the JWT token payload structure
type JWTPayload struct {
	UserID    uint       `json:"userId" validate:"required"`
	Email     string     `json:"email"  validate:"required,email"`
	IssuedAt  time.Time  `json:"iat"`
	ExpiresAt *time.Time `json:"exp"`
	Issuer    string     `json:"iss"`
	Audience  string     `json:"aud"`
}
