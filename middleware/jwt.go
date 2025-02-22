package middleware

import (
	"github.com/golang-jwt/jwt/v5"
)

type JWTCustomClaims struct {
	UserID      int      `json:"user_id"`
	Email       string   `json:"email"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	DeviceID    string   `json:"device_id"`
	jwt.RegisteredClaims
}
