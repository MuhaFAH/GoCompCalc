# GoCompCalc - проект по Go

Маленький итоговый проект состоящий из двух серверов. Один с GUI, предназначенный для пользователя, второй для вычислений. Пользователь вводит на сайте в поле выражение -> результат будет отображен на экране спустя n-ное количество времени (данное время настраивается в пункте **настройки** на сайте)

## ER-Диаграмма
Изображение ER-диаграммы для описания работы находится в файле ER.jpg

![ER](ER.jpg)

## Установка
Для запуска проекта требуется:

1. Клонирование репозитория:
    ```
    git clone https://github.com/MuhaFAH/GoCompCalc.git
    ```
2. Установка необходимых зависимостей:
    ```
    go mod tidy
    ```
3. Запуск:
    ```
    go run cmd/main.go
    ```

4. Перейти на следующий URL: http://localhost:3000 ([Нажми сюда](http://localhost:3000))

Создание базы данных, работа двух серверов (оркестратор и агент) происходят **АВТОМАТИЧЕСКИ**!
Приятного использования **<3**

## Если у вас нет GCC в PATH

При запуске на windows может вылететь ошибка `exec: "gcc": executable file not found in %PATH%`.
Для её решения требуется установить TDM-GCC, это можно сделать через установщик [тыкнув сюда](https://jmeubank.github.io/tdm-gcc/).

## Тестирование

Тесты можно запустить путём следующей командой: 
    ```
    go test ./...
    ```.
  По факту тесты есть только для работы системы пользователей, так как всё остальное очень тесно работает друг с другом + в них всех используется БД, а это очень сложно (реально сложно) тестировать, не хватило времени свободного сделать :х

## Дополнительно и интересно: Для чего и что надо?

1. `cmd/main.go` - отвечает за запуск проекта: активация двух серверов + создание/подключение БД
2. `config/.env` - файл конфигурации, требуется для регулирования количества воркеров-вычислителей. Чем их больше, тем больше параллельно выражений вы сможете вычислить.
3. `pkg/demon` - пакет, отвечающий за работу сервера агента-демона, являющийся обработчиком всех выражений:
    1. __func worker(expressionCh <-chan *Job)__ - горутина, работающая на фоне и ждущая в канале выражений, вычисляет и дает результат в БД либо ошибку
    2. __func statusHandler(w http.ResponseWriter, r *http.Request)__ - дает информацию оркестратору о том, сколько воркеров занято и какие выражения сейчас высчитываются
    3. __func ConnectionCalculateHandler(w http.ResponseWriter, r *http.Request)__ - для веб-сокет связи, получает запрос с выражением и возвращает также результат оркестратору
    4. __func evaluateExpression(expression, id string) (interface{}, error)__ - вычислитель выражения
4. `pkg/orchestrator` - пакет, отвечающий за работу сервера оркестратора. В основном работает с пользователем, даёт ему всю информацию на сайте и принимает от него выражения на обработку. Состоит из:
    1. __func historyHandler(w http.ResponseWriter, r *http.Request)__ - отвечает за страницу с историей выражений, обращаясь за информацией к БД
    2. __func savingHandler(w http.ResponseWriter, r *http.Request)__ - сохранение изменений в времени работы операций, обращается к БД
    3. __func statusHandler(w http.ResponseWriter, r *http.Request)__ - отвечает за страницу статус-контроля воркеров, обращается к агенту для получения информации
    4. __func settingsHandler(w http.ResponseWriter, r *http.Request)__ - отвечает за страницу настроек времени операций
    5. __func mainHandler(w http.ResponseWriter, r *http.Request)__ - отвечает за главную страницу, ввод выражений и вывод результата путём веб-сокет связи
    6. __func registrationHandler(w http.ResponseWriter, r *http.Request)__ - отвечает за страницу регистрации и передачи введенных пользователем данных
    7. __func registerUserHandler(w http.ResponseWriter, r *http.Request)__ - отвечает за валидацию введенных данных, добавление их в БД и выдачу токена новому пользователю
    8. __func loginHandler(w http.ResponseWriter, r *http.Request)__ - отвечает за страницу логина и передачи введенных данных на валидацию через БД
    9. __func verificateLoginUserHandler(w http.ResponseWriter, r *http.Request)__ - отвечает за валидацию введенных данных, проверку логина, токина и пароля
5. `storage/sqlite` - пакет, отвечающий за базу данных. В нём происходит её создание при первой активации сайта + создание таблиц, а после подключение уже существующей к системе