package env

import (
	"log"
	"os"
	"strconv"
)

// GetStr retrieves a string value from the environment variables.
// If the variable is not set, it returns the provided default value and logs a message.
func GetStr(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Printf("Environment variable %s not set, using default value: %s", key, defaultValue)
		return defaultValue
	}
	return value
}

// GetBool retrieves a boolean value from the environment variables.
// If the variable is not set, it returns the provided default value and logs a message.
func GetBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		log.Printf("Environment variable %s not set, using default value: %t", key, defaultValue)
		return defaultValue
	}

	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		log.Printf("Invalid boolean value for environment variable %s: %s, using default value: %t", key, value, defaultValue)
		return defaultValue
	}
	return boolValue
}
