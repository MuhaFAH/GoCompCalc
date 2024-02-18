package demon

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Knetic/govaluate"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/cors"
)

var (
	maxWorkers = 10 // Максимальное количество горутин
	agents     = make(map[string]*ExpressionJob)
	mutex      sync.Mutex
	upgrader   = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	expressionCh = make(chan *ExpressionJob, maxWorkers)
)

type ExpressionJob struct {
	ID         string
	Expression string
	Result     interface{}
}

func RunServer() {
	c := cors.Default()
	for i := 0; i < maxWorkers; i++ {
		go worker(expressionCh)
	}

	http.HandleFunc("/calculate", func(w http.ResponseWriter, r *http.Request) {
		handleRequest(w, r, expressionCh)
	})

	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/ws", handleWebSocket)

	handler := c.Handler(http.DefaultServeMux)
	go func() {
		fmt.Println("Демон успешно запущен на порту: 8080...")
		http.ListenAndServe(":8080", handler)
	}()
	checkExpressions()
}

func handleRequest(w http.ResponseWriter, r *http.Request, expressionCh chan<- *ExpressionJob) {
	log.Println("Received request")

	fmt.Println(22)
	if r.Method != http.MethodPost {
		fmt.Println("Method", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	fmt.Println(1)
	var data map[string]string
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	fmt.Println(2)
	expression, ok := data["expression"]
	if !ok {
		http.Error(w, "Expression not found in JSON", http.StatusBadRequest)
		return
	}
	fmt.Println(3)
	id := uuid.New().String()
	expressionCh <- &ExpressionJob{ID: id, Expression: expression}
	response := map[string]interface{}{
		"status":     "expression sent for evaluation",
		"expression": expression,
		"id":         id,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func checkExpressions() {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Println("ОШИБКА ОТКРЫТИЯ БД 44")
		return
	}
	defer db.Close()
	data, err := db.Query(`
	SELECT key, expression
	FROM Expressions
	WHERE status = 'в обработке'`)
	for data.Next() {
		var id, expr string
		err := data.Scan(&id, &expr)
		if err != nil {
			fmt.Println(err)
		}
		expressionCh <- &ExpressionJob{ID: id, Expression: expr}
	}
	if err != nil {
		fmt.Println("ОШИБКА")
		return
	}
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Println("ОШИБКА ОТКРЫТИЯ БД 5")
		return
	}
	defer db.Close()
	mutex.Lock()
	data, err := db.Query(`
	SELECT expression
	FROM Expressions
	WHERE status = 'в обработке'`)
	if err != nil {
		fmt.Println("ОШИБКА ВЫБОРА ОПЕРАЦИЙ")
		return
	}
	mutex.Unlock()
	in_progress := []string{}
	for data.Next() {
		var expr string
		err := data.Scan(&expr)
		if err != nil {
			fmt.Println(err)
		}
		in_progress = append(in_progress, expr)
	}
	freeWorkers := maxWorkers - len(in_progress)
	statusData := map[string]interface{}{
		"free_workers":           freeWorkers,
		"expressions_in_process": in_progress,
		"max_workers":            maxWorkers,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(statusData)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Println("ОШИБКА ОТКРЫТИЯ БД 22")
		return
	}
	defer db.Close()
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		data := make(map[string]string)
		err = json.Unmarshal(message, &data)
		if err != nil {
			log.Println(err)
			continue
		}

		expression, ok1 := data["expression"]
		agent, ok2 := data["agent_id"]
		if !ok1 && !ok2 {
			log.Println("Expression not found in JSON")
			continue
		}
		if ok2 {
			var (
				result string
				expr   string
			)
			data := db.QueryRow("SELECT result, expression FROM Expressions WHERE key = ?", agent)
			data.Scan(&result, &expr)
			if len(result) > 0 {
				response := map[string]interface{}{
					"result":     result,
					"expression": expr,
					"id":         agent,
				}
				err = conn.WriteJSON(response)
				if err != nil {
					log.Println(err)
				}
			}
			continue
		}
		id := uuid.New().String()
		expressionCh <- &ExpressionJob{ID: id, Expression: expression}
		db, err := sql.Open("sqlite3", "data.db")
		if err != nil {
			fmt.Println("ОШИБКА ОТКРЫТИЯ БД 10")
			return
		}
		defer db.Close()

		query := fmt.Sprintf("INSERT INTO Expressions (key, expression, status, error_message) VALUES ('%s', '%s', 'в обработке', 'nil')", id, expression)
		_, err = db.Exec(query)
		if err != nil {
			fmt.Println("ОШИБКА ДОБАВКИ ВЫРАЖЕНИЯ В ОБРАБОТКЕ В БД:", err)
			return
		}
		response := map[string]interface{}{
			"status":     "expression sent for evaluation",
			"expression": expression,
			"id":         id,
		}

		err = conn.WriteJSON(response)
		if err != nil {
			log.Println(err)
			continue
		}
	}
}

func worker(expressionCh <-chan *ExpressionJob) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Println("ОШИБКА ОТКРЫТИЯ БД 5")
		return
	}
	defer db.Close()
	for {
		time.Sleep(5 * time.Second)
		job := <-expressionCh

		result, err := evaluateExpression(job.Expression, job.ID)
		if err != nil {
			fmt.Printf("Error evaluating expression for job %s: %s\n", job.ID, err)
			continue
		}
		mutex.Lock()
		_, err = db.Exec(fmt.Sprintf("UPDATE Expressions SET completed_at = CURRENT_TIMESTAMP, result = '%s', status = '%s' WHERE key = '%s'", fmt.Sprintf("%v", result), "выполнено", job.ID))
		if err != nil {
			fmt.Println("ОШИБКА РЕЗУЛЬТАТИРОВАНИЯ ВЫРАЖЕНИЯ")
			return
		}
		mutex.Unlock()
	}
}

func evaluateExpression(expression, id string) (interface{}, error) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Println("ОШИБКА ОТКРЫТИЯ БД 5")
		return nil, fmt.Errorf("1")
	}
	defer db.Close()

	data, err := db.Query(`
	SELECT execution_time
	FROM Operations`)
	if err != nil {
		fmt.Println("ОШИБКА ВЫБОРА ОПЕРАЦИЙ")
		return nil, fmt.Errorf("2")
	}
	operations := []int{}
	for data.Next() {
		var time int
		err := data.Scan(&time)
		if err != nil {
			fmt.Println(err)
		}
		operations = append(operations, time)
	}

	expression = strings.ReplaceAll(expression, "^", "**")

	plus_time := operations[0] * strings.Count(expression, "+")
	minus_time := operations[1] * strings.Count(expression, "-")
	multiple_time := operations[2] * strings.Count(expression, "*")
	division_time := operations[3] * strings.Count(expression, "/")

	expr, err := govaluate.NewEvaluableExpression(expression)
	if err != nil {
		return nil, err
	}

	result, err := expr.Evaluate(nil)
	if err != nil {
		return nil, err
	}
	fmt.Println(time.Duration(plus_time+minus_time+multiple_time+division_time) * time.Second)
	time.Sleep(time.Duration(plus_time+minus_time+multiple_time+division_time) * time.Second)
	return result, nil
}
