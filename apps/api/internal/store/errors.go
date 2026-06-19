package store

import "errors"

var (
	ErrNotFound       = errors.New("resource not found")
	ErrDuplicateEmail = errors.New("email already exists")
	ErrDatabase       = errors.New("database operation failed")
)
