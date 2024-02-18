package sqlite

import (
	"database/sql"
	"fmt"
	"os"
)

func NewOrCreateDB(path string) (*sql.DB, error) {
	if _, err := os.Stat(path); err == nil {
		fmt.Println("Таблица подключена.")
		return sql.Open("sqlite3", path)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, nil
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
		return nil, err
	}
	query = `
	CREATE TABLE IF NOT EXISTS Operations (
		id INTEGER PRIMARY KEY,
		operation_type TEXT NOT NULL,
		execution_time INTEGER NOT NULL
	);`
	_, err = db.Exec(query)
	if err != nil {
		return nil, err
	}
	query = `
	INSERT INTO Operations (operation_type, execution_time) VALUES
    ('+', 100),
    ('-', 100),
    ('*', 100),
    ('/', 100);`
	_, err = db.Exec(query)
	if err != nil {
		return nil, err
	}
	query = `
	INSERT INTO Expressions (key, expression, result, status, created_at, completed_at, error_message)
	VALUES 
    ('111', '2 + 2', '4', 'выполнено', '2024-02-14 12:00:00', '2024-02-14 12:00:01', 'nil'),
    ('112', '5 * 3', '15', 'выполнено', '2024-02-14 12:05:00', '2024-02-14 12:05:01', 'nil'),
    ('113', '10 / 2', '5', 'выполнено', '2024-02-14 12:10:00', '2024-02-14 12:10:01', 'nil'),
    ('114', '8 - 4', '4', 'выполнено', '2024-02-14 12:15:00', '2024-02-14 12:15:01', 'nil');`
	_, err = db.Exec(query)
	if err != nil {
		return nil, err
	}
	fmt.Println("Таблица успешно создана.")
	return db, nil
}
