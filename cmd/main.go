package main

import (
	"fmt"
	"gocompcalc/pkg/demon"
	"gocompcalc/pkg/orchestrator"
	"gocompcalc/storage/sqlite"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

// запуск сервера
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {

	fmt.Println("-|---- ЗАПУСК СЕРВЕРА ---|-")

	// создание или подключение БД
	db, err := sqlite.NewOrCreateDB("data.db")
	if err != nil {
		return err
	}
	db.Close()
	// включаем агента
	demon.RunServer()
	// включаем оркестратора
	orchestrator.RunServer()

	return nil
}
