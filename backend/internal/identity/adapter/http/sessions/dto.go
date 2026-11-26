package sessions

// LoginRequest defines credentials for authentication.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email" example:"user@example.com"`
	Password string `json:"password" binding:"required" example:"SecureP@ss123"`
}

// CurrentUserResponse returns the authenticated user profile.
type CurrentUserResponse struct {
	ID            string   `json:"id" example:"usr_12345"`
	UserID        string   `json:"userId" example:"usr_12345"`
	Email         string   `json:"email" example:"user@example.com"`
	Nickname      string   `json:"nickname" example:"alice"`
	EmailVerified bool     `json:"emailVerified" example:"true"`
	Status        string   `json:"status" example:"active"`
	AccountStatus string   `json:"accountStatus" example:"active"`
	Roles         []string `json:"roles" example:"student"`
	Version       int      `json:"version" example:"1"`
}
