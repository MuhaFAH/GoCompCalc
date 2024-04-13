package users

import (
	"database/sql"
	"fmt"
	"regexp"
	"time"

	"github.com/dgrijalva/jwt-go"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

var JWTKey = []byte("MuhaTopSecretKey")

type User struct {
	ID       int
	Name     string
	Password string
}

func LoginValidation(login string) bool {
	if len(login) > 16 {
		return false
	}
	matched, err := regexp.MatchString("^[a-zA-Z_]*$", login)
	if err != nil {
		return false
	}
	return matched
}

func HashingPassword(password string) string {
	saltedBytes := []byte(password)
	hashedBytes, err := bcrypt.GenerateFromPassword(saltedBytes, bcrypt.DefaultCost)
	if err != nil {
		fmt.Printf("ошибка при хэшировании пароля: %v", err)
		return ""
	}
	hash := string(hashedBytes[:])
	return hash
}

func CheckPasswordWithHash(login, password string) bool {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Printf("ошибка подключения БД при проверке пароля: %v", err)
		return false
	}
	defer db.Close()

	var hashedPassword string

	query := "SELECT hashed_password FROM Users WHERE login = ?"
	err = db.QueryRow(query, login).Scan(&hashedPassword)

	if err != nil {
		fmt.Printf("ошибка при получении хэшированного пароля: %v", err)
		return false
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))

	if err != nil {
		fmt.Printf("ошибка при сравнении пароля и хэша: %v", err)
		return false
	}

	return true
}

func ComparePasswords(hashedPwd string, plainPwd string) bool {
	byteHash := []byte(hashedPwd)
	plainPwdByte := []byte(plainPwd)
	err := bcrypt.CompareHashAndPassword(byteHash, plainPwdByte)
	if err != nil {
		fmt.Println("Ошибка при сравнении паролей:", err)
		return false
	}

	return true
}

func GetNewTokenJWT(login string) string {
	expirationTime := time.Now().Add(24 * time.Hour)

	claims := &jwt.StandardClaims{
		Subject:   login,
		ExpiresAt: expirationTime.Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(JWTKey)

	if err != nil {
		fmt.Printf("ошибка при создании токена: %v", err)
		return ""
	}

	return tokenString
}

func ValidateTokenJWT(tokenStr string) bool {
	claims := &jwt.StandardClaims{}

	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return JWTKey, nil
	})

	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			return false
		} else {
			fmt.Printf("Фигня при парсинге лол")
			return false
		}
	}

	if !token.Valid {
		// Если токен недействителен, перенаправляем на страницу регистрации
		return false
	}

	return true
}

func GetUserLogin(tokenStr string) string {
	claims := &jwt.StandardClaims{}

	_, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return JWTKey, nil
	})

	if err != nil {
		if err == jwt.ErrSignatureInvalid {
			return ""
		} else {
			fmt.Printf("Фигня при парсинге лол")
			return ""
		}
	}

	return claims.Subject
}
