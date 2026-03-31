package storage

import "errors"

var (
	// ErrNotFound is returned when a requested key does not exist in the store
	ErrNotFound = errors.New("key not found")
)
