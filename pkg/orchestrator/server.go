package orchestrator

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"text/template"
	"time"
)

func RunServer() {
	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	mux.HandleFunc("/", mainHandler)
	mux.HandleFunc("/history/", historyHandler)
	mux.HandleFunc("/saving/", savingHandler)
	mux.HandleFunc("/settings/", settingsHandler)
	mux.HandleFunc("/status/", statusHandler)

	fmt.Println("ЗАПУСК ОРКЕСТРАТОРА (ПОРТ 3000): SUCCESS")
	http.ListenAndServe(":3000", mux)
}

type Expression struct {
	Status      string
	Result      string
	Expression  string
	CreatedAt   time.Time
	CompletedAt time.Time
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
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

	process_rows, err := db.Query("SELECT status, expression, created_at FROM Expressions WHERE status = 'в обработке'")
	if err != nil {
		fmt.Printf("ошибка получения не обработанных выражений из БД: %v", err)
		return
	}
	for process_rows.Next() {
		var expression Expression
		err := process_rows.Scan(&expression.Status, &expression.Expression, &expression.CreatedAt)
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

	finished_rows, err := db.Query("SELECT status, result, expression, created_at, completed_at FROM Expressions WHERE status = 'выполнено'")
	if err != nil {
		fmt.Printf("ошибка получения выполненных выражений из БД: %v", err)
		return
	}
	for finished_rows.Next() {
		var expression Expression
		err := finished_rows.Scan(&expression.Status, &expression.Result, &expression.Expression, &expression.CreatedAt, &expression.CompletedAt)
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

	error_rows, err := db.Query("SELECT status, expression, created_at, completed_at FROM Expressions WHERE status = 'ошибка'")
	if err != nil {
		fmt.Printf("ошибка получения неверных выражений из БД: %v", err)
		return
	}
	for error_rows.Next() {
		var expression Expression
		err := error_rows.Scan(&expression.Status, &expression.Expression, &expression.CreatedAt, &expression.CompletedAt)
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

func savingHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Printf("ошибка подключения БД в сохранении операций: %v", err)
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
		fmt.Printf("Ошибка обновления (+): %v", err)
		return
	}

	_, err = db.Exec(query, time_minus, "-")
	if err != nil {
		fmt.Printf("Ошибка обновления (-): %v", err)
		return
	}

	_, err = db.Exec(query, time_multiple, "*")
	if err != nil {
		fmt.Printf("Ошибка обновления (*): %v", err)
		return
	}

	_, err = db.Exec(query, time_division, "/")
	if err != nil {
		fmt.Printf("Ошибка обновления (/): %v", err)
		return
	}

	http.Redirect(w, r, "/settings", http.StatusMovedPermanently)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	html_tmpl, err := template.ParseFiles("web/status.html", "web/includes/header.html", "web/includes/head.html")
	if err != nil {
		fmt.Printf("ошибка загрузки шаблона статуса воркеров: %v", err)
		return
	}

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
		http.Error(w, "Error decoding JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	html_tmpl.ExecuteTemplate(w, "status", context)
}

func settingsHandler(w http.ResponseWriter, r *http.Request) {
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

	data, err := db.Query("SELECT execution_time FROM Operations")
	if err != nil {
		fmt.Printf("ошибка при получении информации об операциях из БД: %v", err)
		return
	}

	operations := []int{}
	for data.Next() {
		var time int
		err := data.Scan(&time)
		if err != nil {
			fmt.Printf("ошибка загрузки информации операции EH: %v", err)
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
		Division: operations[3],
	}

	html_tmpl.ExecuteTemplate(w, "settings", context)
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	html_tmpl, err := template.ParseFiles("web/main.html", "web/includes/header.html", "web/includes/head.html")
	if err != nil {
		fmt.Printf("ошибка загрузки шаблона главной: %v", err)
		return
	}

	html_tmpl.ExecuteTemplate(w, "main", nil)
}
