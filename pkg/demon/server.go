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
		log.Printf("[WARNING] Файл конфигурации не был загружен: %v", err)
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
		port := os.Getenv("DEMON_PORT")
		log.Printf("[INFO] Запуск демона - SUCCESS [PORT: %s]", port)
		http.ListenAndServe(":"+port, mux)
	}()

	checkNotEndedExpressions()
}

// структура для приходящих выражений
type Job struct {
	ID         string
	Expression string
	Result     interface{}
	User       string
}

// воркер
func worker(expressionCh <-chan *Job) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		log.Printf("[ERROR] База данных не была запущена - WORKER: %v", err)
		return
	}
	defer db.Close()
	// воркеры работают постоянно
	for {
		// ожидаем выражение
		job := <-expressionCh
		// производим вычисление через иную функцию
		result, err := evaluateExpression(job.Expression, job.ID, job.User)
		if err != nil {
			mutex.Lock()
			// если произошла ошибка, то добавляем в БД информацию об этом
			_, err = db.Exec(fmt.Sprintf("UPDATE Expressions SET completed_at = CURRENT_TIMESTAMP, status = '%s' WHERE key = '%s'", "ошибка", job.ID))
			if err != nil {
				log.Printf("[WARNING] Выражение со статусом ОШИБКА не был добавлен в БД: %v", err)
				return
			}
			mutex.Unlock()
			continue
		}
		mutex.Lock()
		// если всё хорошо, то сохраняем результат, время выполнение и меняем статус в БД
		_, err = db.Exec(fmt.Sprintf("UPDATE Expressions SET completed_at = CURRENT_TIMESTAMP, result = '%s', status = '%s' WHERE key = '%s'", fmt.Sprintf("%v", result), "выполнено", job.ID))
		if err != nil {
			log.Printf("[WARNING] Выражение со статусом ВЫПОЛНЕНО не был добавлен в БД: %v", err)
			return
		}

		mutex.Unlock()
	}
}

// функция для проверки на незаконченные выражения, которые остались с прошлой работы сервера
func checkNotEndedExpressions() {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		log.Printf("[ERROR] База данных не была запущена - CHECK_NOT_ENDED: %v", err)
		return
	}
	defer db.Close()
	// получаем незаконченные
	data, err := db.Query("SELECT key, expression, user FROM Expressions WHERE status = 'в обработке'")
	if err != nil {
		log.Printf("[WARNING] Незаконченные выражения не были получены: %v", err)
		return
	}

	for data.Next() {
		var id, expression, user string
		err := data.Scan(&id, &expression, &user)
		if err != nil {
			log.Printf("[WARNING] Незаконченное выражение не было загружено в поток: %v", err)
		}
		// добавляем их в поток, чтобы воркеры их доделали
		expressionCh <- &Job{ID: id, Expression: expression, User: user}
	}
}

// функция для ответа оркестратору статусом воркеров
func statusHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		log.Printf("[ERROR] База данных не была запущена - STATUS_WORKERS: %v", err)
		return
	}
	defer db.Close()

	mutex.Lock()
	// смотрим, какие выражения сейчас вычисляются
	data, err := db.Query("SELECT expression FROM Expressions WHERE status = 'в обработке'")
	if err != nil {
		log.Printf("[WARNING] Вычисляемые выражения не были получены: %v", err)
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
			log.Printf("[WARNING] Информация о воркерах не была получена: %v", err)
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
		log.Printf("[ERROR] Ошибка записи JSON о воркерах: %v", err)
		return
	}
}

// функция для веб-сокет связи с оркестратором и вычисления выражений в потоке
func ConnectionCalculateHandler(w http.ResponseWriter, r *http.Request) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		log.Printf("[ERROR] База данных не была запущена - WEB_SOCKETS: %v", err)
		return
	}
	defer db.Close()
	// устанавливаем связь
	connection, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WARNING] Веб-сокет не были подключен: %v", err)
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
			log.Printf("[WARNING] Не удалось загрузить данные с сокетов: %v", err)
			continue
		}
		// смотрим, есть ли там выражение или просьба получить результат на выражение, которое было отправлено ранее
		expression, ok1 := data["expression"]
		keyForResult, ok2 := data["getresult"]

		if !ok1 && !ok2 {
			log.Printf("[WARNING] В пришедшем запросе нет данных: %v", err)
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
					log.Printf("[ERROR] Не удалось отправить ответные данные о выражении: %v", err)
				}
			}
			continue
		}
		// если прислали выражение, то кидаем его в канал, пусть считают :)
		idForExpression := uuid.New().String()
		userName := data["user"]
		expressionCh <- &Job{ID: idForExpression, Expression: expression, User: userName}
		// добавляем в БД информацию о выражении, что оно сейчас считается
		query := fmt.Sprintf("INSERT INTO Expressions (key, expression, status, error_message, user) VALUES ('%s', '%s', 'в обработке', 'nil', '%s')", idForExpression, expression, userName)
		_, err = db.Exec(query)
		if err != nil {
			log.Printf("[ERROR] Не удалось добавить новое выражение в БД: %v", err)
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
			log.Printf("[ERROR] Не удалось отправить ответ по сокетам: %v", err)
			continue
		}
	}
}

// функция для высчитывания результата выражения
func evaluateExpression(expression, id, user string) (interface{}, error) {
	db, err := sql.Open("sqlite3", "data.db")
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения БД в расчёте: %v", err)
	}
	defer db.Close()
	// пытаемся получить время операций
	data, err := db.Query("SELECT execution_time FROM Operations WHERE user = ?", user)
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
