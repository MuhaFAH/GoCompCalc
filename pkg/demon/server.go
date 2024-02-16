package demon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Knetic/govaluate"
)

func RunServer() {
	go func() {
		http.HandleFunc("/calculate", handleCalculate)
		fmt.Println("Демон успешно запущен на порту: 8080...")
		http.ListenAndServe(":8080", nil)
	}()
}

func handleCalculate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data map[string]string
	err := json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	expression, ok := data["expression"]
	if !ok {
		http.Error(w, "Expression not found in JSON", http.StatusBadRequest)
		return
	}

	result, err := evaluateExpression(expression)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"result": result,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func evaluateExpression(expression string) (interface{}, error) {
	expression = strings.ReplaceAll(expression, "^", "**") // govaluate не поддерживает оператор "^"

	// Создаем новый evaluator
	expr, err := govaluate.NewEvaluableExpression(expression)
	if err != nil {
		return nil, err
	}

	// Вычисляем результат выражения
	result, err := expr.Evaluate(nil)
	if err != nil {
		return nil, err
	}

	return result, nil
}
