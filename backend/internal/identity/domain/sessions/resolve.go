package sessions

import (
	"errors"
)

var (
	ErrInvalidSession  = errors.New("invalid session")
	ErrInactiveAccount = errors.New("inactive account")
)
