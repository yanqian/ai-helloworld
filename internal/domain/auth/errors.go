package auth

import "errors"

// ErrEmailExists indicates a duplicate email address.
var ErrEmailExists = errors.New("email already exists")
