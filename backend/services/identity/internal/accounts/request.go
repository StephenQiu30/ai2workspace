// Package accounts contains request DTOs.
package accounts

// RegisterRequest defines the input for public account registration.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email" example:"user@example.com"`
	Password string `json:"password" binding:"required,min=8" example:"SecureP@ss123"`
}

// VerifyEmailRequest defines the input for email verification with a token.
type VerifyEmailRequest struct {
	Token string `json:"token" binding:"required" example:"a1b2c3d4e5f6"`
}

// PasswordResetRequest defines the input to request a password reset email.
type PasswordResetRequest struct {
	Email string `json:"email" binding:"required,email" example:"user@example.com"`
}

// ResetPasswordRequest defines the input to update a password using a reset token.
type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required" example:"a1b2c3d4e5f6"`
	NewPassword string `json:"newPassword" binding:"required,min=8" example:"NewSecureP@ss123"`
}
