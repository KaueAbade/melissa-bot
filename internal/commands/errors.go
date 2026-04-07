package commands

import (
	"errors"
	"fmt"
)

// Default error for when a command is not found
var ErrCommandNotFound = errors.New("command not found")

var (
	ErrNilCommand                     = errors.New("nil command")
	ErrMissingResponseBuilder         = errors.New("missing response builder")
	ErrNilResponseTemplate            = errors.New("nil response template")
	ErrMissingDefaultResponseTemplate = errors.New("missing default locale response template")
)

func wrapCommandError(key CommandKey, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("command '%s': %w", key, err)
}
