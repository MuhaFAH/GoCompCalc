package orchestrator

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"gocompcalc/pkg/users"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/joho/godotenv"
)

// запуск сервера и всех обработчиков
func RunServer() {
	mux := http.NewServeMux()

	err := godotenv.Load("config/.env")
	if err != nil {
		log.Printf("[WARNING] Файл конфигурации не был загружен: %v", err)
	}

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/logout", logoutUserHandler)
	mux.HandleFunc("/", mainHandler)
	mux.HandleFunc("/history/", historyHandler)
	mux.HandleFunc("/saving/", savingHandler)
	mux.HandleFunc("/settings/", settingsHandler)
	mux.HandleFunc("/status/", statusHandler)
	mux.HandleFunc("/registration/", registrationHandler)
	mux.HandleFunc("/register/", registerUserHandler)
	mux.HandleFunc("/login/", loginHandler)
	mux.HandleFunc("/loginning/", verificateLoginUserHandler)

	port := os.Getenv("ORCHESTRATOR_PORT")
	log.Printf("[INFO] Запуск оркестратора - SUCCESS [PORT: %s]", port)
	http.ListenAndServe(":"+port, mux)
}

// структура для выражения
type Expression struct {
	Status      string
	Result      string
	Expression  string
	CreatedAt   time.Time
	CompletedAt time.Time
	User        string
}

// обработчик для подтверждения входа и валидации данных
func verificateLoginUserHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		log.Printf("[ERROR] Ошибка при подключении базы данных - VERIFICATION: %v", err)
		return
	}
	defer db.Close()

	login := r.FormValue("login")
	password := r.FormValue("password")

	if login == "" || password == "" {
		http.SetCookie(w, &http.Cookie{
			Name:  "login_error",
			Value: "Empty",
			Path:  "/",
		})
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if !users.LoginValidation(login) {
		http.SetCookie(w, &http.Cookie{
			Name:  "login_error",
			Value: "NotFound",
			Path:  "/",
		})
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	var hashedPassword string
	query := "SELECT hashed_password FROM Users WHERE login = ?"
	err = db.QueryRow(query, login).Scan(&hashedPassword)
	if err != nil {
		http.SetCookie(w, &http.Cookie{
			Name:  "login_error",
			Value: "NotFound",
			Path:  "/",
		})
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if !users.ComparePasswords(hashedPassword, password) {
		http.SetCookie(w, &http.Cookie{
			Name:  "login_error",
			Value: "NotFound",
			Path:  "/",
		})
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	usertoken := users.GetNewTokenJWT(login)

	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   usertoken,
		Expires: time.Now().Add(24 * time.Hour),
		Path:    "/",
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

// обработчик для страницы входа пользователя
func loginHandler(w http.ResponseWriter, r *http.Request) {
	html_tmpl, err := template.ParseFiles("web/login.html", "web/includes/header.html", "web/includes/head.html")
	if err != nil {
		log.Printf("[ERROR] Шаблон не загружен - LOGIN: %v", err)
		return
	}

	// Получаем куки
	cookie, err := r.Cookie("login_error")
	var errorMessage string
	if err == nil && cookie != nil {
		// Все возможные ситуации
		if cookie.Value == "NotFound" {
			errorMessage = "Логин или пароль введены неверно >:("
		}
		if cookie.Value == "NotValid" {
			errorMessage = "Ваш логин не соответствует стандартам :("
		}
		if cookie.Value == "Empty" {
			errorMessage = "Не все поля заполнены :|"
		}
		// Удаляем куки после получения сообщения
		http.SetCookie(w, &http.Cookie{
			Name:    "login_error",
			Value:   "",
			Path:    "/",
			Expires: time.Unix(0, 0),
		})
	}

	html_tmpl.ExecuteTemplate(w, "login", map[string]string{"errorMessage": errorMessage})
}

// обработчик для выхода пользователя
func logoutUserHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

// обработчик для подтверждения регистрации и сохранения данных
func registerUserHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		log.Printf("[ERROR] Ошибка при подключении базы данных - REGISTRATION: %v", err)
		return
	}
	defer db.Close()

	// Получаем введенные пользователем данные
	login := r.FormValue("login")
	password := r.FormValue("password")
	pass_conf := r.FormValue("password_confirm")

	// Проверяем все возможные случаи с полями + валидация
	if login == "" || password == "" || pass_conf == "" {
		http.SetCookie(w, &http.Cookie{
			Name:  "registration_error",
			Value: "Empty",
			Path:  "/",
		})
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}
	if password != pass_conf {
		http.SetCookie(w, &http.Cookie{
			Name:  "registration_error",
			Value: "DontMatch",
			Path:  "/",
		})
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}

	var exists bool
	// Проверка на уникальность логина
	query := "SELECT EXISTS (SELECT 1 FROM Users WHERE login = ?)"
	err = db.QueryRow(query, login).Scan(&exists)
	if err != nil {
		log.Printf("[ERROR] Ошибка при проверке логина: %v", err)
		return
	}
	if exists {
		http.SetCookie(w, &http.Cookie{
			Name:  "registration_error",
			Value: "Exists",
			Path:  "/",
		})
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}

	// Соответствует ли логин стандартам
	if !users.LoginValidation(login) {
		http.SetCookie(w, &http.Cookie{
			Name:  "registration_error",
			Value: "NotValid",
			Path:  "/",
		})
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}

	// Если всё хорошо, то хэшируем пароль и заносим в БД пользователя
	hashedPass := users.HashingPassword(password)
	query = fmt.Sprintf("INSERT INTO Users (login, hashed_password) VALUES ('%s', '%s')", login, hashedPass)
	_, err = db.Exec(query)
	if err != nil {
		log.Printf("[ERROR] Не удалось добавить нового пользователя в БД: %v", err)
		return
	}

	// Создаем в БД под пользователя индивидуальные настройки математических операций
	users.CreateUserOperations(login, "data.db")

	// Создаем JWT-токен и кладем пользователю в куки
	usertoken := users.GetNewTokenJWT(login)
	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   usertoken,
		Expires: time.Now().Add(24 * time.Hour),
		Path:    "/",
	})

	http.Redirect(w, r, "/", http.StatusMovedPermanently)
}

// обработчик для страницы регистрации пользователя
func registrationHandler(w http.ResponseWriter, r *http.Request) {
	html_tmpl, err := template.ParseFiles("web/registration.html", "web/includes/header.html", "web/includes/head.html")
	if err != nil {
		log.Printf("[ERROR] Шаблон не загружен - REGISTRATION: %v", err)
		return
	}
	// Получаем куки, если при регистрации что-то не прошло проверку - сообщаем пользователю
	cookie, err := r.Cookie("registration_error")
	var errorMessage string
	if err == nil && cookie != nil {
		if cookie.Value == "Exists" {
			errorMessage = "Данный логин уже занят <3"
		}
		if cookie.Value == "NotValid" {
			errorMessage = "Ваш логин не соответствует стандартам: от 5 до 16 символов, без цифр"
		}
		if cookie.Value == "DontMatch" {
			errorMessage = "Пароли не совпадают!"
		}
		if cookie.Value == "Empty" {
			errorMessage = "Не все поля заполнены :|"
		}
		// Удаляем куки после получения сообщения
		http.SetCookie(w, &http.Cookie{
			Name:    "registration_error",
			Value:   "",
			Path:    "/",
			Expires: time.Unix(0, 0),
		})
	}

	html_tmpl.ExecuteTemplate(w, "registration", map[string]string{"errorMessage": errorMessage})
}

// обработчик для страницы истории выражений
func historyHandler(w http.ResponseWriter, r *http.Request) {
	// Проверка, что есть JWT-токен и что он валидный
	cookie, err := r.Cookie("token")
	if err != nil {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}
	if !users.ValidateTokenJWT(cookie.Value, "data.db") {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}

	html_tmpl, err := template.ParseFiles("web/history.html", "web/includes/header.html", "web/includes/head.html")
	if err != nil {
		log.Printf("[ERROR] Шаблон не загружен - HISTORY: %v", err)
		return
	}

	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		log.Printf("[ERROR] Ошибка при подключении базы данных - HISTORY: %v", err)
		return
	}
	defer db.Close()

	var expressions []Expression

	process_rows, err := db.Query("SELECT status, expression, created_at, user FROM Expressions WHERE status = 'в обработке'")
	if err != nil {
		log.Printf("[ERROR] Ошибка получения необработанных выражений из БД: %v", err)
		return
	}
	for process_rows.Next() {
		var expression Expression
		err := process_rows.Scan(&expression.Status, &expression.Expression, &expression.CreatedAt, &expression.User)
		if err != nil {
			log.Printf("[ERROR] Ошибка загрузки необработанного выражения: %v", err)
			return
		}
		expressions = append(expressions, expression)
	}
	if err := process_rows.Err(); err != nil {
		log.Printf("[ERROR] Ошибка необработанного выражения: %v", err)
		return
	}
	process_rows.Close()

	finished_rows, err := db.Query("SELECT status, result, expression, created_at, completed_at, user FROM Expressions WHERE status = 'выполнено'")
	if err != nil {
		log.Printf("[ERROR] Ошибка получения выполненных выражений из БД: %v", err)
		return
	}
	for finished_rows.Next() {
		var expression Expression
		err := finished_rows.Scan(&expression.Status, &expression.Result, &expression.Expression, &expression.CreatedAt, &expression.CompletedAt, &expression.User)
		if err != nil {
			log.Printf("[ERROR] Ошибка загрузки выполненного выражения: %v", err)
			return
		}
		expressions = append(expressions, expression)
	}
	if err := finished_rows.Err(); err != nil {
		log.Printf("[ERROR] Ошибка выполненного выражения: %v", err)
		return
	}
	finished_rows.Close()

	error_rows, err := db.Query("SELECT status, expression, created_at, completed_at, user FROM Expressions WHERE status = 'ошибка'")
	if err != nil {
		log.Printf("[ERROR] Ошибка получения неверных выражений из БД: %v", err)
		return
	}
	for error_rows.Next() {
		var expression Expression
		err := error_rows.Scan(&expression.Status, &expression.Expression, &expression.CreatedAt, &expression.CompletedAt, &expression.User)
		if err != nil {
			log.Printf("[ERROR] Ошибка загрузки неверного выражения: %v", err)
			return
		}
		expressions = append(expressions, expression)
	}
	if err := error_rows.Err(); err != nil {
		log.Printf("[ERROR] Ошибка неверного выражения: %v", err)
		return
	}
	error_rows.Close()

	html_tmpl.ExecuteTemplate(w, "history", expressions)
}

// обработчик для сохранения изменения времени операций
func savingHandler(w http.ResponseWriter, r *http.Request) {
	// Проверка, что есть JWT-токен и что он валидный
	cookie, err := r.Cookie("token")
	if err != nil {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}
	if !users.ValidateTokenJWT(cookie.Value, "data.db") {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}

	login := users.GetUserLogin(cookie.Value)

	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		log.Printf("[ERROR] Ошибка при подключении базы данных - SAVING_OPERATIONS: %v", err)
		return
	}
	defer db.Close()

	// получаем указанное пользователем время операций
	time_plus := r.FormValue("plus")
	time_minus := r.FormValue("minus")
	time_multiple := r.FormValue("multiple")
	time_division := r.FormValue("division")

	// обновляем в БД информацию
	query := `
	UPDATE Operations 
	SET execution_time = ? 
	WHERE operation_type = ? AND user = ?
	`
	_, err = db.Exec(query, time_plus, "+", login)
	if err != nil {
		log.Printf("[WARNING] Ошибка обновления (+): %v", err)
		return
	}

	_, err = db.Exec(query, time_minus, "-", login)
	if err != nil {
		log.Printf("[WARNING] Ошибка обновления (-): %v", err)
		return
	}

	_, err = db.Exec(query, time_multiple, "*", login)
	if err != nil {
		log.Printf("[WARNING] Ошибка обновления (*): %v", err)
		return
	}

	_, err = db.Exec(query, time_division, "/", login)
	if err != nil {
		log.Printf("[WARNING] Ошибка обновления (/): %v", err)
		return
	}
	// делаем редирект на эту же страницу
	http.Redirect(w, r, "/settings", http.StatusMovedPermanently)
}

// обработчик для страницы наблюдения за процессами и воркерами
func statusHandler(w http.ResponseWriter, r *http.Request) {
	// Проверка, что есть JWT-токен и что он валидный
	cookie, err := r.Cookie("token")
	if err != nil {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}
	if !users.ValidateTokenJWT(cookie.Value, "data.db") {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}

	html_tmpl, err := template.ParseFiles("web/status.html", "web/includes/header.html", "web/includes/head.html")
	if err != nil {
		log.Printf("[ERROR] Шаблон не загружен - STATUS_WORKERS: %v", err)
		return
	}

	// делаем запрос для информации
	response, err := http.Get("http://localhost:8080/status")
	if err != nil {
		log.Printf("[ERROR] Не удалось получить информацию о воркерах: %v", err)
		return
	}
	defer response.Body.Close()

	var context struct {
		FreeWorkers int      `json:"free_workers"`
		MaxWorkers  int      `json:"max_workers"`
		Expressions []string `json:"expressions_in_process"`
	}

	if err := json.NewDecoder(response.Body).Decode(&context); err != nil {
		log.Printf("[ERROR] Не удалось декодировать: %v", err)
		return
	}

	// передаем информацию в шаблон контекстом
	html_tmpl.ExecuteTemplate(w, "status", context)
}

// обработчик для страницы времени операций
func settingsHandler(w http.ResponseWriter, r *http.Request) {
	// Проверка, что есть JWT-токен и что он валидный
	cookie, err := r.Cookie("token")
	if err != nil {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}
	if !users.ValidateTokenJWT(cookie.Value, "data.db") {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}

	html_tmpl, err := template.ParseFiles("web/settings.html", "web/includes/header.html", "web/includes/head.html")
	if err != nil {
		log.Printf("[ERROR] Шаблон не загружен - SETTINGS: %v", err)
		return
	}

	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		log.Printf("[ERROR] Ошибка при подключении базы данных - SETTINGS: %v", err)
		return
	}
	defer db.Close()

	// получаем актуальное время для заполнения полей
	data, err := db.Query("SELECT execution_time FROM Operations WHERE user = ?", users.GetUserLogin(cookie.Value))
	if err != nil {
		log.Printf("[ERROR] Ошибка при получении информации об операциях из БД: %v", err)
		return
	}

	// сохраняем время для дальнейшего использования
	operations := []int{}
	for data.Next() {
		var time int
		err := data.Scan(&time)
		if err != nil {
			log.Printf("[ERROR] Ошибка при загрузке времени операций: %v", err)
		}
		operations = append(operations, time)
	}

	// передаём время контекстом в шаблон
	context := struct {
		Plus     int
		Minus    int
		Multiple int
		Division int
	}{
		Plus:     operations[0],
		Minus:    operations[1],
		Multiple: operations[2],
		Division: operations[3],
	}

	html_tmpl.ExecuteTemplate(w, "settings", context)
}

// обработчик для главной страницы
func mainHandler(w http.ResponseWriter, r *http.Request) {
	// Проверка, что есть JWT-токен и что он валидный
	cookie, err := r.Cookie("token")
	token := cookie.Value
	if err != nil {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}
	if !users.ValidateTokenJWT(token, "data.db") {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}

	html_tmpl, err := template.ParseFiles("web/main.html", "web/includes/header.html", "web/includes/head.html")
	if err != nil {
		log.Printf("[ERROR] Шаблон не загружен - MAIN: %v", err)
		return
	}

	html_tmpl.ExecuteTemplate(w, "main", users.GetUserLogin(token))
}
