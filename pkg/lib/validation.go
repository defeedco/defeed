package lib

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

var goValidator = validator.New()

// ValidationErrors represents multiple validation errors.
type ValidationErrors struct {
	Errors []string `json:"errors"`
}

// Error implements the error interface.
func (ve ValidationErrors) Error() string {
	if len(ve.Errors) == 0 {
		return "no validation errors"
	}

	return strings.Join(ve.Errors, "; ")
}

// ValidateStruct validates a struct using go-playground/validator and returns a slice of errors.
// When validation passes, it returns nil.
func ValidateStruct(s any) error {
	if err := goValidator.Struct(s); err != nil {
		if ve, ok := err.(validator.ValidationErrors); ok {
			out := ValidationErrors{Errors: []string{err.Error()}}
			for _, e := range ve {
				out.Errors = append(out.Errors, fmt.Sprintf("%s %s", e.Field(), e.ActualTag()))
			}
			return out
		}
		return err
	}
	return nil
}
