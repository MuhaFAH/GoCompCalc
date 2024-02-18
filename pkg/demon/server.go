package demon

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Knetic/govaluate"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

var (
	mutex        sync.Mutex
	expressionCh chan *Job
	upgrader     = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

func RunServer() {
	err := godotenv.Load("config/.env")
	if err != nil {
		fmt.Printf("ошибка загрузки файла конфигурации: %v", err)
	}
	c := cors.Default()

	maxWorkers, err := strconv.Atoi(os.Getenv("COUNT_WORKERS"))
	if err != nil {
		maxWorkers = 5
	}
	expressionCh = make(chan *Job, maxWorkers)
	for i := 0; i < maxWorkers; i++ {
		go worker(expressionCh)
	}

	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/ws", ConnectionCalculateHandler)

	mux := c.Handler(http.DefaultServeMux)
	go func() {
		fmt.Println("ЗАПУСК ДЕМОНА-АГЕНТА (ПОРТ 8080): SUCCESS")
		http.ListenAndServe(":8080", mux)
	}()

	checkNotEndedExpressions()
}

type Job struct {
	ID         string
	Expression string
	Result     interface{}
}

func worker(expressionCh <-chan *Job) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Printf("ошибка подключения БД в воркере: %v", err)
		return
	}
	defer db.Close()

	for {
		job := <-expressionCh

		result, err := evaluateExpression(job.Expression, job.ID)
		if err != nil {
			mutex.Lock()

			_, err = db.Exec(fmt.Sprintf("UPDATE Expressions SET completed_at = CURRENT_TIMESTAMP, status = '%s' WHERE key = '%s'", "ошибка", job.ID))
			if err != nil {
				fmt.Printf("ошибка добавления записи в БД о неверном выражении: %v", err)
				return
			}
			mutex.Unlock()
			continue
		}
		mutex.Lock()

		_, err = db.Exec(fmt.Sprintf("UPDATE Expressions SET completed_at = CURRENT_TIMESTAMP, result = '%s', status = '%s' WHERE key = '%s'", fmt.Sprintf("%v", result), "выполнено", job.ID))
		if err != nil {
			fmt.Printf("ошибка добавления записи в БД о выполненном выражении: %v", err)
			return
		}

		mutex.Unlock()
	}
}

func checkNotEndedExpressions() {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Printf("ошибка подключения БД в проверке выражений: %v", err)
		return
	}
	defer db.Close()

	data, err := db.Query("SELECT key, expression FROM Expressions WHERE status = 'в обработке'")
	if err != nil {
		fmt.Printf("ошибка получения информации о незаконченных выражениях: %v", err)
	}

	for data.Next() {
		var id, expression string
		err := data.Scan(&id, &expression)
		if err != nil {
			fmt.Printf("ошибка загрузки незаконченного выражения: %v", err)
		}
		expressionCh <- &Job{ID: id, Expression: expression}
	}
	if err != nil {
		fmt.Printf("ошибка незаконченного выражения: %v", err)
		return
	}
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Printf("ошибка подключения БД в запросе о статусе воркеров: %v", err)
		return
	}
	defer db.Close()

	mutex.Lock()
	data, err := db.Query("SELECT expression FROM Expressions WHERE status = 'в обработке'")
	if err != nil {
		fmt.Printf("ошибка получения информации для статуса воркеров: %v", err)
		return
	}
	mutex.Unlock()

	inProcessWorkers := []string{}

	for data.Next() {
		var expression string
		err := data.Scan(&expression)
		if err != nil {
			fmt.Printf("ошибка загрузки информации для статуса воркеров: %v", err)
		}
		inProcessWorkers = append(inProcessWorkers, expression)
	}

	maxWorkers, err := strconv.Atoi(os.Getenv("COUNT_WORKERS"))
	if err != nil {
		maxWorkers = 5
	}
	freeWorkers := maxWorkers - len(inProcessWorkers)
	if freeWorkers < 0 {
		freeWorkers = 0
	}
	structData := struct {
		FreeWorkers int      `json:"free_workers"`
		MaxWorkers  int      `json:"max_workers"`
		InProcess   []string `json:"expressions_in_process"`
	}{FreeWorkers: freeWorkers,
		MaxWorkers: maxWorkers,
		InProcess:  inProcessWorkers}

	jsonData, _ := json.Marshal(structData)
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(jsonData)
	if err != nil {
		fmt.Printf("ошибка записи JSON о воркерах: %v", err)
		return
	}
}

func ConnectionCalculateHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Printf("ошибка подключения БД в веб-сокетах: %v", err)
		return
	}
	defer db.Close()

	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("ошибка подключения сокетов: %v", err)
		return
	}
	defer connection.Close()

	for {
		_, message, err := connection.ReadMessage()
		if err != nil {
			return
		}

		data := make(map[string]string)
		err = json.Unmarshal(message, &data)
		if err != nil {
			fmt.Printf("не удалось загрузить данные: %v", err)
			continue
		}

		expression, ok1 := data["expression"]
		keyForResult, ok2 := data["getresult"]

		if !ok1 && !ok2 {
			fmt.Printf("в запросе не найдена информация о выражении: %v", err)
			continue
		}
		if ok2 {

			var (
				result     string
				expression string
			)

			data := db.QueryRow("SELECT result, expression FROM Expressions WHERE key = ?", keyForResult)
			data.Scan(&result, &expression)

			if len(result) > 0 {
				response := map[string]interface{}{
					"result":     result,
					"expression": expression,
					"id":         keyForResult,
				}

				err = connection.WriteJSON(response)
				if err != nil {
					fmt.Printf("не удалось отправить ответные данные о выражении: %v", err)
				}
			}
			continue
		}

		idForExpression := uuid.New().String()
		expressionCh <- &Job{ID: idForExpression, Expression: expression}

		query := fmt.Sprintf("INSERT INTO Expressions (key, expression, status, error_message) VALUES ('%s', '%s', 'в обработке', 'nil')", idForExpression, expression)
		_, err = db.Exec(query)
		if err != nil {
			fmt.Printf("не удалось добавить новое выражение в БД: %v", err)
			return
		}

		response := map[string]interface{}{
			"status":     "в обработке",
			"expression": expression,
			"id":         idForExpression,
		}

		err = connection.WriteJSON(response)
		if err != nil {
			log.Println(err)
			continue
		}
	}
}

func evaluateExpression(expression, id string) (interface{}, error) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения БД в расчёте: %v", err)
	}
	defer db.Close()

	data, err := db.Query("SELECT execution_time FROM Operations")
	if err != nil {
		return nil, fmt.Errorf("не удалось запросить время операций из БД: %v", err)
	}

	operations := []int{}
	for data.Next() {
		var time int
		err := data.Scan(&time)
		if err != nil {
			return nil, fmt.Errorf("ошибка загрузки операций из БД: %v", err)
		}
		operations = append(operations, time)
	}

	plus_time := operations[0] * strings.Count(expression, "+")
	minus_time := operations[1] * strings.Count(expression, "-")
	multiple_time := operations[2] * strings.Count(expression, "*")
	division_time := operations[3] * strings.Count(expression, "/")

	expr, err := govaluate.NewEvaluableExpression(expression)
	if err != nil {
		return nil, fmt.Errorf("не удалось преобразовать в выражение: %v", err)
	}

	result, err := expr.Evaluate(nil)
	if err != nil {
		return nil, fmt.Errorf("не удалось провести расчёт выражения: %v", err)
	}

	timing := time.Duration(plus_time+minus_time+multiple_time+division_time) * time.Millisecond

	if timing < 1*time.Millisecond {
		timing = time.Millisecond
	}

	time.Sleep(timing)
	return result, nil
}
