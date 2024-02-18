package sqlite

import (
	"database/sql"
	"fmt"
	"os"
)

func NewOrCreateDB(path string) (*sql.DB, error) {

	if _, err := os.Stat(path); err == nil {
		fmt.Println("Подключение таблицы: SUCCESS")
		return sql.Open("sqlite3", path)
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("ошибка при создании БД: %v", err)
	}

	query := `
	CREATE TABLE IF NOT EXISTS Expressions (
		id INTEGER PRIMARY KEY,
		key TEXT NOT NULL,
		expression TEXT NOT NULL,
		result TEXT,
		status TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		completed_at TIMESTAMP,
		error_message TEXT
	);`
	_, err = db.Exec(query)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания таблицы выражений: %v", err)
	}

	query = `
	CREATE TABLE IF NOT EXISTS Operations (
		id INTEGER PRIMARY KEY,
		operation_type TEXT NOT NULL,
		execution_time INTEGER NOT NULL
	);`
	_, err = db.Exec(query)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания таблицы операторов: %v", err)
	}

	query = `
	INSERT INTO Operations (operation_type, execution_time) VALUES
    ('+', 100),
    ('-', 100),
    ('*', 100),
    ('/', 100);`
	_, err = db.Exec(query)
	if err != nil {
		return nil, fmt.Errorf("ошибка заполнения таблицы операторов: %v", err)
	}

	fmt.Println("Создание таблицы: SUCCESS")
	return db, nil
}
