package orchestrator

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"gocompcalc/pkg/users"
	"net/http"
	"text/template"
	"time"
)

// запуск сервера и всех обработчиков
func RunServer() {
	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/logoutuser/", logoutUserHandler)
	mux.HandleFunc("/", mainHandler)
	mux.HandleFunc("/history/", historyHandler)
	mux.HandleFunc("/saving/", savingHandler)
	mux.HandleFunc("/settings/", settingsHandler)
	mux.HandleFunc("/status/", statusHandler)
	mux.HandleFunc("/registration/", registrationHandler)
	mux.HandleFunc("/register/", registerUserHandler)
	mux.HandleFunc("/login/", loginHandler)
	mux.HandleFunc("/loginning/", verificateLoginUserHandler)

	fmt.Println("ЗАПУСК ОРКЕСТРАТОРА (ПОРТ 3000): SUCCESS")
	http.ListenAndServe(":3000", mux)
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

func verificateLoginUserHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Printf("ошибка подключения БД при входе пользователя: %v", err)
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

	http.Redirect(w, r, "/", http.StatusMovedPermanently)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	html_tmpl, err := template.ParseFiles("web/login.html", "web/includes/header.html", "web/includes/head.html")
	if err != nil {
		fmt.Printf("ошибка парсинга шаблона логина: %v", err)
		return
	}

	cookie, err := r.Cookie("login_error")
	var errorMessage string
	if err == nil && cookie != nil {
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

	html_tmpl.ExecuteTemplate(w, "login", map[string]string{
		"errorMessage": errorMessage,
	})
}

func logoutUserHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   "",
		Path:    "/",
		Expires: time.Unix(0, 0),
	})

	http.Redirect(w, r, "/", http.StatusMovedPermanently)
}

func registerUserHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Printf("ошибка подключения БД при регистрации пользователя: %v", err)
		return
	}
	defer db.Close()

	login := r.FormValue("login")
	password := r.FormValue("password")
	pass_conf := r.FormValue("password_confirm")

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

	query := "SELECT EXISTS (SELECT 1 FROM Users WHERE login = ?)"
	err = db.QueryRow(query, login).Scan(&exists)

	if err != nil {
		fmt.Printf("ошибка при проверке логина: %v", err)
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

	if !users.LoginValidation(login) {
		http.SetCookie(w, &http.Cookie{
			Name:  "registration_error",
			Value: "NotValid",
			Path:  "/",
		})
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}

	hashedPass := users.HashingPassword(password)

	query = fmt.Sprintf("INSERT INTO Users (login, hashed_password) VALUES ('%s', '%s')", login, hashedPass)
	_, err = db.Exec(query)

	if err != nil {
		fmt.Printf("не удалось добавить нового пользователя в БД: %v", err)
		return
	}

	users.CreateUserOperations(login)

	usertoken := users.GetNewTokenJWT(login)

	http.SetCookie(w, &http.Cookie{
		Name:    "token",
		Value:   usertoken,
		Expires: time.Now().Add(24 * time.Hour),
		Path:    "/",
	})

	http.Redirect(w, r, "/", http.StatusMovedPermanently)
}

func registrationHandler(w http.ResponseWriter, r *http.Request) {
	html_tmpl, err := template.ParseFiles("web/registration.html", "web/includes/header.html", "web/includes/head.html")
	if err != nil {
		fmt.Printf("ошибка парсинга шаблона регистрации: %v", err)
		return
	}
	cookie, err := r.Cookie("registration_error")
	var errorMessage string
	if err == nil && cookie != nil {
		if cookie.Value == "Exists" {
			errorMessage = "Данный логин уже занят <3"
		}
		if cookie.Value == "NotValid" {
			errorMessage = "Ваш логин не соответствует стандартам :("
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
	html_tmpl.ExecuteTemplate(w, "registration", map[string]string{
		"errorMessage": errorMessage,
	})
}

// обработчик для страницы истории выражений
func historyHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("token")
	if err != nil {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}
	if !users.ValidateTokenJWT(cookie.Value) {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}

	html_tmpl, err := template.ParseFiles("web/history.html", "web/includes/header.html", "web/includes/head.html")
	if err != nil {
		fmt.Printf("ошибка парсинга шаблона истории выражений: %v", err)
		return
	}

	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Printf("ошибка подключения БД в истории выражений: %v", err)
		return
	}
	defer db.Close()

	var expressions []Expression

	process_rows, err := db.Query("SELECT status, expression, created_at, user FROM Expressions WHERE status = 'в обработке'")
	if err != nil {
		fmt.Printf("ошибка получения не обработанных выражений из БД: %v", err)
		return
	}
	for process_rows.Next() {
		var expression Expression
		err := process_rows.Scan(&expression.Status, &expression.Expression, &expression.CreatedAt, &expression.User)
		if err != nil {
			fmt.Printf("ошибка загрузки не обработанного выражения: %v", err)
			return
		}
		expressions = append(expressions, expression)
	}
	if err := process_rows.Err(); err != nil {
		fmt.Printf("ошибка не обработанного выражения: %v", err)
		return
	}
	process_rows.Close()

	finished_rows, err := db.Query("SELECT status, result, expression, created_at, completed_at, user FROM Expressions WHERE status = 'выполнено'")
	if err != nil {
		fmt.Printf("ошибка получения выполненных выражений из БД: %v", err)
		return
	}
	for finished_rows.Next() {
		var expression Expression
		err := finished_rows.Scan(&expression.Status, &expression.Result, &expression.Expression, &expression.CreatedAt, &expression.CompletedAt, &expression.User)
		if err != nil {
			fmt.Printf("ошибка загрузки выполненного выражения: %v", err)
			return
		}
		expressions = append(expressions, expression)
	}
	if err := finished_rows.Err(); err != nil {
		fmt.Printf("ошибка выполненного выражения: %v", err)
		return
	}
	finished_rows.Close()

	error_rows, err := db.Query("SELECT status, expression, created_at, completed_at, user FROM Expressions WHERE status = 'ошибка'")
	if err != nil {
		fmt.Printf("ошибка получения неверных выражений из БД: %v", err)
		return
	}
	for error_rows.Next() {
		var expression Expression
		err := error_rows.Scan(&expression.Status, &expression.Expression, &expression.CreatedAt, &expression.CompletedAt, &expression.User)
		if err != nil {
			fmt.Printf("ошибка загрузки неверного выражения: %v", err)
			return
		}
		expressions = append(expressions, expression)
	}
	if err := error_rows.Err(); err != nil {
		fmt.Printf("ошибка неверного выражения: %v", err)
		return
	}
	error_rows.Close()

	html_tmpl.ExecuteTemplate(w, "history", expressions)
}

// обработчик для сохранения изменения времени операций
func savingHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("token")
	if err != nil {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}
	if !users.ValidateTokenJWT(cookie.Value) {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}

	login := users.GetUserLogin(cookie.Value)

	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Printf("ошибка подключения БД в сохранении операций: %v", err)
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
		fmt.Printf("Ошибка обновления (+): %v", err)
		return
	}

	_, err = db.Exec(query, time_minus, "-", login)
	if err != nil {
		fmt.Printf("Ошибка обновления (-): %v", err)
		return
	}

	_, err = db.Exec(query, time_multiple, "*", login)
	if err != nil {
		fmt.Printf("Ошибка обновления (*): %v", err)
		return
	}

	_, err = db.Exec(query, time_division, "/", login)
	if err != nil {
		fmt.Printf("Ошибка обновления (/): %v", err)
		return
	}
	// делаем редирект на эту же страницу
	http.Redirect(w, r, "/settings", http.StatusMovedPermanently)
}

// обработчик для страницы наблюдения за процессами и воркерами
func statusHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("token")
	if err != nil {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}
	if !users.ValidateTokenJWT(cookie.Value) {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}

	html_tmpl, err := template.ParseFiles("web/status.html", "web/includes/header.html", "web/includes/head.html")
	if err != nil {
		fmt.Printf("ошибка загрузки шаблона статуса воркеров: %v", err)
		return
	}
	// делаем запрос для информации
	response, err := http.Get("http://localhost:8080/status")
	if err != nil {
		fmt.Printf("не удалось получить информацию о воркерах: %v", err)
		return
	}
	defer response.Body.Close()

	var context struct {
		FreeWorkers int      `json:"free_workers"`
		MaxWorkers  int      `json:"max_workers"`
		Expressions []string `json:"expressions_in_process"`
	}

	if err := json.NewDecoder(response.Body).Decode(&context); err != nil {
		fmt.Printf("не удалось декодировать: %v", err)
		return
	}
	// передаем информацию в шаблон контекстом
	html_tmpl.ExecuteTemplate(w, "status", context)
}

// обработчик для страницы времени операций
func settingsHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("token")
	if err != nil {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}
	if !users.ValidateTokenJWT(cookie.Value) {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}

	html_tmpl, err := template.ParseFiles("web/settings.html", "web/includes/header.html", "web/includes/head.html")
	if err != nil {
		fmt.Printf("ошибка подключения шаблона в настройках: %v", err)
		return
	}

	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Printf("ошибка подключения БД в настройках: %v", err)
		return
	}
	defer db.Close()
	// получаем актуальное время для заполнения полей
	data, err := db.Query("SELECT execution_time FROM Operations WHERE user = ?", users.GetUserLogin(cookie.Value))
	if err != nil {
		fmt.Printf("ошибка при получении информации об операциях из БД: %v", err)
		return
	}
	// сохраняем время для дальнейшего использования
	operations := []int{}
	for data.Next() {
		var time int
		err := data.Scan(&time)
		if err != nil {
			fmt.Printf("ошибка загрузки информации операции EH: %v", err)
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
	cookie, err := r.Cookie("token")
	token := cookie.Value
	if err != nil {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}
	if !users.ValidateTokenJWT(token) {
		http.Redirect(w, r, "/registration", http.StatusFound)
		return
	}
	html_tmpl, err := template.ParseFiles("web/main.html", "web/includes/header.html", "web/includes/head.html")
	if err != nil {
		fmt.Printf("ошибка загрузки шаблона главной: %v", err)
		return
	}
	html_tmpl.ExecuteTemplate(w, "main", users.GetUserLogin(token))
}
