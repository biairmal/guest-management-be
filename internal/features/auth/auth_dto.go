package auth

// LoginInput is the request body for POST /api/v1/auth/login.
//
// swagger:model LoginInput
type LoginInput struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// RefreshInput is the request body for POST /api/v1/auth/refresh.
//
// swagger:model RefreshInput
type RefreshInput struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// TokenPair is the response for a successful login or refresh.
//
// swagger:model TokenPair
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}
