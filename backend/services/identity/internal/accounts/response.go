// Package accounts contains response DTOs.
package accounts

// RegisterResponse represents the result of an account registration.
type RegisterResponse struct {
	Accepted          bool   `json:"accepted" example:"true"`
	NextStep          string `json:"nextStep" example:"verify_email"`
	VerificationToken string `json:"verificationToken,omitempty" example:"a1b2c3d4e5f6"`
}

// VerifyEmailResponse represents the result of email verification.
type VerifyEmailResponse struct {
	EmailVerified bool `json:"emailVerified" example:"true"`
}

// ResendVerificationResponse represents the result of requesting a new verification email.
type ResendVerificationResponse struct {
	Accepted          bool   `json:"accepted" example:"true"`
	NextStep          string `json:"nextStep" example:"verify_email"`
	VerificationToken string `json:"verificationToken,omitempty" example:"a1b2c3d4e5f6"`
}

// PasswordResetResponse represents the outcome of a password reset request.
type PasswordResetResponse struct {
	Accepted   bool   `json:"accepted" example:"true"`
	ResetToken string `json:"resetToken,omitempty" example:"a1b2c3d4e5f6"`
}

// ResetPasswordResponse represents the completion of a password reset.
type ResetPasswordResponse struct {
	Updated bool `json:"updated" example:"true"`
}

// ErrorResponse conveys failure details to the client without exposing internals.
type ErrorResponse struct {
	Error     string `json:"error" example:"bad_request"`
	Message   string `json:"message" example:"Invalid input provided"`
	RequestID string `json:"requestId,omitempty" example:"req_123456"`
}
