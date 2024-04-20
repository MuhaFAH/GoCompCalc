package users

import (
	"database/sql"
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/dgrijalva/jwt-go"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

// Ключ безопасности
var JWTKey = []byte("MuhaTopSecretKey")

// Валидация логина (стандарты)
func LoginValidation(login string) bool {
	if len(login) > 16 {
		return false
	}
	if len(login) < 5 {
		return false
	}

	check, err := regexp.MatchString("^[a-zA-Z_]*$", login)
	if err != nil {
		return false
	}

	return check
}

// Хэширование пароля
func HashingPassword(password string) string {
	passBytes := []byte(password)
	hashedBytes, err := bcrypt.GenerateFromPassword(passBytes, bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[ERROR] Ошибка при хэшировании пароля пользователя: %v", err)
		return ""
	}

	hash := string(hashedBytes[:])
	return hash
}

// Сравнение хэшированного пароля и пароля, введенного пользователем
func ComparePasswords(hash string, password string) bool {
	byteHash := []byte(hash)
	bytePassword := []byte(password)

	err := bcrypt.CompareHashAndPassword(byteHash, bytePassword)

	return err == nil
}

// Выдача индивидуального токена пользователю
func GetNewTokenJWT(login string) string {
	expirationTime := time.Now().Add(24 * time.Hour)

	claims := &jwt.StandardClaims{
		Subject:   login,
		ExpiresAt: expirationTime.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(JWTKey)

	if err != nil {
		log.Printf("[ERROR] Ошибка при создании токена для пользователя: %v", err)
		return ""
	}

	return tokenString
}

// Валидация имеющегося у пользователя токена
func ValidateTokenJWT(tokenStr, database string) bool {
	db, err := sql.Open("sqlite3", database)
	if err != nil {
		log.Printf("[ERROR] Ошибка при подключении базы данных - TOKEN_VALIDATION: %v", err)
		return false
	}
	defer db.Close()

	claims := &jwt.StandardClaims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return JWTKey, nil
	})

	if err != nil {
		return false
	}

	if !token.Valid {
		return false
	}

	var exists bool
	query := "SELECT EXISTS (SELECT 1 FROM Users WHERE login = ?)"
	err = db.QueryRow(query, claims.Subject).Scan(&exists)
	if err != nil {
		log.Printf("[ERROR] Ошибка при проверке токена на существование логина: %v", err)
		return false
	}
	if !exists {
		return false
	}

	return true
}

// Получение логина пользователя по его токену
func GetUserLogin(tokenStr string) string {
	claims := &jwt.StandardClaims{}

	_, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return JWTKey, nil
	})

	if err != nil {
		return ""
	}

	return claims.Subject
}

// Создание математических операций для определенного пользователя
func CreateUserOperations(login, database string) {
	db, err := sql.Open("sqlite3", database)
	if err != nil {
		log.Printf("[ERROR] Ошибка при подключении базы данных - USER_OPERATIONS: %v", err)
		return
	}
	defer db.Close()

	query := fmt.Sprintf("INSERT INTO Operations (operation_type, execution_time, user) VALUES ('+', 100, '%s'), ('-', 100, '%s'), ('*', 100, '%s'), ('/', 100, '%s')", login, login, login, login)

	_, err = db.Exec(query)
	if err != nil {
		log.Printf("[ERROR] Ошибка заполнения таблицы операторов пользователя: %v", err)
		return
	}
}
