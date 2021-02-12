package kiterrno

import (
	"fmt"
	"strings"
)

const (
	// Predefined error code categories
	UNREGISTERED int = iota // unregistered error types
	THRIFT                  // native thrift application exception codes (will be wrapped as KITE error codes)
	KITE                    // error codes used by KITE framework
	MESH                    // error codes used by mesh proxy
)

// ErrorCategory represents a set of error codes and their description.
// Frameword extension developers may define and register their own category.
type ErrorCategory struct {
	Name     string         // The simple description of the category
	CodeDesc map[int]string // The error codes and their descriptions
}

// RegisterErrors is for defining and registering custom error code category globally.
// Return nil if registration succeeds, error if it fails.
// This function is **not thread-safe** and should be called only in the initialization phase.
func RegisterErrors(name string, category int, codes map[int]string) error {
	if codes == nil {
		return fmt.Errorf("Codes is nil")
	}

	if c, ok := registeredErrors[category]; ok {
		return fmt.Errorf("Category %d conflicts with: %s", category, c.Name)
	}

	// The errSep will be used in error formatting thus should not appear in the category name.
	if strings.Contains(name, errSep) {
		return fmt.Errorf("Category name should not contains '%s': %s", errSep, name)
	}

	registeredErrors[category] = &ErrorCategory{name, codes} // should we make a copy of codes?

	return nil
}

// registeredErrors contains all globally registered error categories.
var registeredErrors map[int]*ErrorCategory

// QueryErrorCode find and return the category name and the description of the given error code
func QueryErrorCode(category, errno int) (name, desc string) {
	if c := registeredErrors[category]; c != nil {
		if d, ok := c.CodeDesc[errno]; ok {
			name, desc = c.Name, d
		} else {
			name, desc = c.Name, "?"
		}
	} else {
		name, desc = registeredErrors[UNREGISTERED].Name, "?"
	}
	return
}

// IsUnregisteredCategory determine if the result of `QueryErrorCode` is an unregistered category name.
func IsUnregisteredCategory(name string) bool {
	return name == registeredErrors[UNREGISTERED].Name
}
