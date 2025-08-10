package utils

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

var goValidator = validator.New()

// ValidateStruct validates a struct using go-playground/validator and returns a slice of errors.
// When validation passes, it returns nil.
func ValidateStruct(s any) []error {
	if err := goValidator.Struct(s); err != nil {
		if ve, ok := err.(validator.ValidationErrors); ok {
			out := make([]error, 0, len(ve))
			for _, e := range ve {
				out = append(out, fmt.Errorf("%s %s", e.Field(), e.ActualTag()))
			}
			return out
		}
		return []error{err}
	}
	return nil
}
