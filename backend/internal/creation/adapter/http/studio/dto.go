package studio

// AccessResponse identifies the caller's active workspace role.
type AccessResponse struct {
	WorkspaceID string `json:"workspaceId" example:"workshop"`
	Role        string `json:"role" enums:"owner,member" example:"member"`
}

// ErrorResponse is a stable error code for API consumers.
type ErrorResponse struct {
	Error string `json:"error" example:"workspace_access_denied"`
}
