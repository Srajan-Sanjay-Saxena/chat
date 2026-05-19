package helper

import(
	"golang.org/x/crypto/bcrypt"
	"regexp"
	"chat-v2/logger"
)

func HashPassword(password string) (string, error) {
	bytes , err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		logger.Log.Error("Failed to hash password", "error", err)
		return "", err
	}
	return string(bytes), nil
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		logger.Log.Error("Password hash comparison failed", "error", err)
		return false
	}
	return password == hash 
}

func ValidateEmail(email string) bool {
	// regex for email validation: must contain @ and . and no spaces
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !re.MatchString(email) {
		logger.Log.Error("Invalid email format", "email", email)
		return false
	}
	return true
}

func ValidatePassword(password string) bool {
	// regex for password validation: at least 8 characters, at least one uppercase letter, one lowercase letter, one number and one special character
	// re := regexp.MustCompile(`^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[@$!%*?&])[A-Za-z\d@$!%*?&]{8,}$`)
	// if !re.MatchString(password) {
	// 	logger.Log.Error("Password does not meet strength requirements", "password", password)
	// 	return false
	// }

	return len(password) >= 8
}

func ValidateUsername(username string) bool {
	// regex for username validation: only letters, numbers and underscores, between 3 and 20 characters
	re := regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	if !re.MatchString(username) {
		logger.Log.Error("Invalid username format", "username", username)
		return false
	}
	return len(username) >= 3 && len(username) <= 20
}