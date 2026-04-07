package commands

import (
	"errors"
)

// Default error for when a command is not found
var ErrCommandNotFound = errors.New("command not found")
