package helper

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
)

var Validate *validator.Validate

func init() {
	Validate = validator.New()

	// Register custom validation for username: alphanumeric + underscore
	Validate.RegisterValidation("alphanum_underscore", validateAlphanumUnderscore)
}

func validateAlphanumUnderscore(fl validator.FieldLevel) bool {
	for _, c := range fl.Field().String() {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// ValidateRequest validates a struct and returns a user-friendly error message
func ValidateRequest(req interface{}) error {
	if err := Validate.Struct(req); err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			for _, e := range validationErrors {
				switch e.Field() {
				case "Username":
					switch e.Tag() {
					case "required":
						return fmt.Errorf("username is required")
					case "min":
						return fmt.Errorf("username must be at least 3 characters")
					case "max":
						return fmt.Errorf("username must be at most 20 characters")
					case "alphanum_underscore":
						return fmt.Errorf("username can only contain letters, numbers and underscores")
					}
				case "Password":
					switch e.Tag() {
					case "required":
						return fmt.Errorf("password is required")
					case "min":
						return fmt.Errorf("password must be at least 8 characters")
					}
				case "Email":
					switch e.Tag() {
					case "required":
						return fmt.Errorf("email is required")
					case "email":
						return fmt.Errorf("invalid email format")
					}
				case "Title":
					switch e.Tag() {
					case "required":
						return fmt.Errorf("title is required")
					case "min":
						return fmt.Errorf("title must be at least 1 character")
					case "max":
						return fmt.Errorf("title must be at most 100 characters")
					}
				case "Content":
					switch e.Tag() {
					case "required":
						return fmt.Errorf("content is required")
					case "min":
						return fmt.Errorf("content must be at least 1 character")
					case "max":
						return fmt.Errorf("content is too long")
					}
				}
			}
		}
		return fmt.Errorf("invalid request")
	}
	return nil
}
