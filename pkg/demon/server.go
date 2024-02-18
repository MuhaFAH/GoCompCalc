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
	// подключаем файл конфигурации
	err := godotenv.Load("config/.env")
	if err != nil {
		fmt.Printf("ошибка загрузки файла конфигурации: %v", err)
	}
	// подключаем протоколы CORS (заголовки)
	c := cors.Default()
	// берем число воркеров-вычислителей из файла конфигурации
	maxWorkers, err := strconv.Atoi(os.Getenv("COUNT_WORKERS"))
	if err != nil {
		maxWorkers = 5
	}
	// создаем канал для выражений, которые будут приходить от оркестратора
	expressionCh = make(chan *Job, maxWorkers)
	// запускаем на фон наших вычислителей
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

// структура для приходящих выражений
type Job struct {
	ID         string
	Expression string
	Result     interface{}
}

// воркер
func worker(expressionCh <-chan *Job) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Printf("ошибка подключения БД в воркере: %v", err)
		return
	}
	defer db.Close()
	// воркеры работают постоянно
	for {
		// ожидаем выражение
		job := <-expressionCh
		// производим вычисление через иную функцию
		result, err := evaluateExpression(job.Expression, job.ID)
		if err != nil {
			mutex.Lock()
			// если произошла ошибка, то добавляем в БД информацию об этом
			_, err = db.Exec(fmt.Sprintf("UPDATE Expressions SET completed_at = CURRENT_TIMESTAMP, status = '%s' WHERE key = '%s'", "ошибка", job.ID))
			if err != nil {
				fmt.Printf("ошибка добавления записи в БД о неверном выражении: %v", err)
				return
			}
			mutex.Unlock()
			continue
		}
		mutex.Lock()
		// если всё хорошо, то сохраняем результат, время выполнение и меняем статус в БД
		_, err = db.Exec(fmt.Sprintf("UPDATE Expressions SET completed_at = CURRENT_TIMESTAMP, result = '%s', status = '%s' WHERE key = '%s'", fmt.Sprintf("%v", result), "выполнено", job.ID))
		if err != nil {
			fmt.Printf("ошибка добавления записи в БД о выполненном выражении: %v", err)
			return
		}

		mutex.Unlock()
	}
}

// функция для проверки на незаконченные выражения, которые остались с прошлой работы сервера
func checkNotEndedExpressions() {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Printf("ошибка подключения БД в проверке выражений: %v", err)
		return
	}
	defer db.Close()
	// получаем не законченные
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
		// добавляем их в поток, чтобы воркеры их доделали
		expressionCh <- &Job{ID: id, Expression: expression}
	}
	if err != nil {
		fmt.Printf("ошибка незаконченного выражения: %v", err)
		return
	}
}

// функция для ответа оркестратору статусом воркеров
func statusHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Printf("ошибка подключения БД в запросе о статусе воркеров: %v", err)
		return
	}
	defer db.Close()

	mutex.Lock()
	// смотрим, какие выражения сейчас вычисляются
	data, err := db.Query("SELECT expression FROM Expressions WHERE status = 'в обработке'")
	if err != nil {
		fmt.Printf("ошибка получения информации для статуса воркеров: %v", err)
		return
	}
	mutex.Unlock()
	// слайс для выражений, над которыми сейчас работают воркеры
	inProcessWorkers := []string{}
	// загружаем выражения
	for data.Next() {
		var expression string
		err := data.Scan(&expression)
		if err != nil {
			fmt.Printf("ошибка загрузки информации для статуса воркеров: %v", err)
		}
		inProcessWorkers = append(inProcessWorkers, expression)
	}
	// получаем информацию о том, сколько всего воркеров и сколько свободно
	maxWorkers, err := strconv.Atoi(os.Getenv("COUNT_WORKERS"))
	if err != nil {
		maxWorkers = 5
	}
	freeWorkers := maxWorkers - len(inProcessWorkers)
	if freeWorkers < 0 {
		freeWorkers = 0
	}
	// делаем JSON со всей информацией
	structData := struct {
		FreeWorkers int      `json:"free_workers"`
		MaxWorkers  int      `json:"max_workers"`
		InProcess   []string `json:"expressions_in_process"`
	}{FreeWorkers: freeWorkers,
		MaxWorkers: maxWorkers,
		InProcess:  inProcessWorkers}

	jsonData, _ := json.Marshal(structData)
	// отправляем на другой сервер
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(jsonData)
	if err != nil {
		fmt.Printf("ошибка записи JSON о воркерах: %v", err)
		return
	}
}

// функция для веб-сокет связи с оркестратором и вычисления выражений в потоке
func ConnectionCalculateHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		fmt.Printf("ошибка подключения БД в веб-сокетах: %v", err)
		return
	}
	defer db.Close()
	// устанавливаем связь
	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("ошибка подключения сокетов: %v", err)
		return
	}
	defer connection.Close()

	for {
		// ожидаем информацию
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
		// смотрим, есть ли там выражение или просьба получить результат на выражение, которое было отправлено ранее
		expression, ok1 := data["expression"]
		keyForResult, ok2 := data["getresult"]

		if !ok1 && !ok2 {
			fmt.Printf("в запросе не найдена информация о выражении: %v", err)
			continue
		}
		// если просят дать результат на ранее присланное выражение
		if ok2 {

			var (
				result     string
				expression string
			)
			// получаем выражение по присланному ID
			data := db.QueryRow("SELECT result, expression FROM Expressions WHERE key = ?", keyForResult)
			data.Scan(&result, &expression)
			// если результат есть, шлём
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
		// если прислали выражение, то кидаем его в канал, пусть считают :)
		idForExpression := uuid.New().String()
		expressionCh <- &Job{ID: idForExpression, Expression: expression}
		// добавляем в БД информацию о выражении, что оно сейчас считается
		query := fmt.Sprintf("INSERT INTO Expressions (key, expression, status, error_message) VALUES ('%s', '%s', 'в обработке', 'nil')", idForExpression, expression)
		_, err = db.Exec(query)
		if err != nil {
			fmt.Printf("не удалось добавить новое выражение в БД: %v", err)
			return
		}
		// отвечаем тем, что всё хорошо, всё пришло и мы уже считаем
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

// функция для высчитывания результата выражения
func evaluateExpression(expression, id string) (interface{}, error) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения БД в расчёте: %v", err)
	}
	defer db.Close()
	// пытаемся получить время операций
	data, err := db.Query("SELECT execution_time FROM Operations")
	if err != nil {
		return nil, fmt.Errorf("не удалось запросить время операций из БД: %v", err)
	}
	// загружаем время операций
	operations := []int{}
	for data.Next() {
		var time int
		err := data.Scan(&time)
		if err != nil {
			return nil, fmt.Errorf("ошибка загрузки операций из БД: %v", err)
		}
		operations = append(operations, time)
	}
	// считаем сколько в общей сумме потребуется времени
	plus_time := operations[0] * strings.Count(expression, "+")
	minus_time := operations[1] * strings.Count(expression, "-")
	multiple_time := operations[2] * strings.Count(expression, "*")
	division_time := operations[3] * strings.Count(expression, "/")
	// создаем выражение
	expr, err := govaluate.NewEvaluableExpression(expression)
	if err != nil {
		return nil, fmt.Errorf("не удалось преобразовать в выражение: %v", err)
	}
	// производим расчёт
	result, err := expr.Evaluate(nil)
	if err != nil {
		return nil, fmt.Errorf("не удалось провести расчёт выражения: %v", err)
	}

	timing := time.Duration(plus_time+minus_time+multiple_time+division_time) * time.Millisecond
	// если вообще нет знаков операций :(
	if timing < 1*time.Millisecond {
		timing = time.Millisecond
	}
	// тянем на рассчитанное время
	time.Sleep(timing)
	return result, nil
}
