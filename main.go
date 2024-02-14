package main

import (
	"database/sql"
	"fmt"
	"gocompcalc/storage/sqlite"
	"html/template"
	"net/http"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Expression struct {
	Status      string
	Expression  string
	CreatedAt   time.Time
	CompletedAt time.Time
}

func history(w http.ResponseWriter, r *http.Request) {
	templ, err := template.ParseFiles("web/history.html", "web/includes/header.html", "web/includes/head.html")
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Println("ОШИБКА ПОДКЛЮЧЕНИЯ БД 3")
		return
	}
	defer db.Close()
	rows, err := db.Query("SELECT status, expression, created_at, completed_at FROM Expressions")
	if err != nil {
		fmt.Println("Ошибка выполнения запроса к базе данных:", err)
		return
	}
	defer rows.Close()
	var expressions []Expression
	for rows.Next() {
		var expression Expression
		err := rows.Scan(&expression.Status, &expression.Expression, &expression.CreatedAt, &expression.CompletedAt)
		if err != nil {
			fmt.Println("Ошибка сканирования строки из результата запроса:", err)
			return
		}
		fmt.Println(expression)
		expressions = append(expressions, expression)
	}
	if err := rows.Err(); err != nil {
		fmt.Println("Ошибка получения результатов запроса:", err)
		return
	}
	templ.ExecuteTemplate(w, "history", expressions)
}
func save_operations(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Println("ОШИБКА СОХРАНЕНИЯ ДАННЫХ ОПЕРАЦИЙ")
		return
	}
	defer db.Close()

	time_plus := r.FormValue("plus")
	time_minus := r.FormValue("minus")
	time_multiple := r.FormValue("multiple")
	time_division := r.FormValue("division")

	query := `
	UPDATE Operations 
	SET execution_time = ? 
	WHERE operation_type = ?
	`
	_, err = db.Exec(query, time_plus, "+")
	if err != nil {
		fmt.Println("ОШИБКА ОБНОВЛЕНИЯ ДАННЫХ ОПЕРАЦИЙ 1")
		return
	}
	_, err = db.Exec(query, time_minus, "-")
	if err != nil {
		fmt.Println("ОШИБКА ОБНОВЛЕНИЯ ДАННЫХ ОПЕРАЦИЙ 2")
		return
	}
	_, err = db.Exec(query, time_multiple, "*")
	if err != nil {
		fmt.Println("ОШИБКА ОБНОВЛЕНИЯ ДАННЫХ ОПЕРАЦИЙ 3")
		return
	}
	_, err = db.Exec(query, time_division, "/")
	if err != nil {
		fmt.Println("ОШИБКА ОБНОВЛЕНИЯ ДАННЫХ ОПЕРАЦИЙ 4")
		return
	}
	http.Redirect(w, r, "/settings", 301)
}

func settingsHandler(w http.ResponseWriter, r *http.Request) {
	templ, err := template.ParseFiles("web/settings.html", "web/includes/header.html", "web/includes/head.html")
	if err != nil {
		fmt.Println("ОШИБКА В ЗАГРУЗКЕ ШАБЛОНА: НАСТРОЙКИ")
		return
	}
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Println("ОШИБКА ОТКРЫТИЯ БД 2")
		return
	}
	data, err := db.Query(`
	SELECT execution_time 
	FROM Operations`)
	operations := []int{}
	for data.Next() {
		var time int
		err := data.Scan(&time)
		if err != nil {
			fmt.Println(err)
		}
		operations = append(operations, time)
	}
	context := struct {
		Plus     int
		Minus    int
		Multiple int
		Division int
	}{
		Plus:     operations[0],
		Minus:    operations[1],
		Multiple: operations[2],
		Division: operations[3]}
	templ.ExecuteTemplate(w, "settings", context)
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	templ, err := template.ParseFiles("web/main.html", "web/includes/header.html", "web/includes/head.html")
	if err != nil {
		fmt.Println("ОШИБКА В ЗАГРУЗКЕ ШАБЛОНА: ГЛАВНАЯ")
		return
	}

	templ.ExecuteTemplate(w, "main", nil)
}

func main() {
	db, err := sqlite.NewOrCreateDB("data.db")
	if err != nil {
		fmt.Println("ОШИБКА БД:", err)
		return
	}
	defer db.Close()

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	mux.HandleFunc("/", mainHandler)
	mux.HandleFunc("/settings/", settingsHandler)
	mux.HandleFunc("/save_operations", save_operations)
	mux.HandleFunc("/history", history)

	fmt.Println("Запуск сервера...")
	http.ListenAndServe(":"+port, mux)
}