package commands

import (
	"errors"
	"fmt"
)

// These are definitions for common errors that can occur during command processing
var (
	ErrCommandNotFound                = errors.New("command not found")
	ErrNilCommand                     = errors.New("nil command")
	ErrMissingResponseBuilder         = errors.New("missing response builder")
	ErrNilResponseTemplate            = errors.New("nil response template")
	ErrMissingDefaultResponseTemplate = errors.New("missing default locale response template")
)

// wrapCommandError is a helper function that wraps an error with the command key for better context in error messages.
func wrapCommandError(key CommandKey, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("command '%s': %w", key, err)
}
