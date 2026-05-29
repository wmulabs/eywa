package errors

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrConflict       = errors.New("conflict")
	ErrRitualTerminal = errors.New("ritual already in terminal status")
)

type NotFoundError struct {
	Entity string
	ID     string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found", e.Entity, e.ID)
}

// Is makes errors.Is(err, ErrNotFound) work even when wrapped.
func (e *NotFoundError) Is(target error) bool {
	return target == ErrNotFound
}

type ConflictError struct {
	Entity string
	ID     string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s %q already exists", e.Entity, e.ID)
}

func (e *ConflictError) Is(target error) bool {
	return target == ErrConflict
}
