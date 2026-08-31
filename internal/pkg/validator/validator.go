package validator

import (
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterValidation("alphanum_underscore", validateAlphanumUnderscore)
}

func validateAlphanumUnderscore(fl validator.FieldLevel) bool {
	for _, c := range fl.Field().String() {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

func Validate(req any) error {
	if err := validate.Struct(req); err != nil {
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
				}
			}
		}
		return fmt.Errorf("invalid request")
	}
	return nil
}
