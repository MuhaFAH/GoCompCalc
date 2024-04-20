В этом дополнительном материале мы с вами узнаем, как можно хранить пароли в СУБД.
В финальном проекте вам предстоит хранить данные пользователя для его аутентификации. 
Встаёт вопрос, в каком виде хранить пароли?

### Настраиваем проект
Создаём проект
```
go mod init psswd_lesson
```

Добавляем пакет для работы с SQLite
```
go get github.com/mattn/go-sqlite3
```

Добавляем пакет для работы с пакетом для шифрования
```
go get golang.org/x/crypto/bcrypt
```

в папке имеем 2 файла   
**go.mod**
```
module psswd_lesson

go 1.22.1

require (
	github.com/mattn/go-sqlite3 v1.14.22 // indirect
	golang.org/x/crypto v0.21.0 // indirect
)

```

**go.sum**
```
github.com/mattn/go-sqlite3 v1.14.22 h1:2gZY6PC6kBnID23Tichd1K+Z0oS6nE/XwU+Vz/5o4kU=
github.com/mattn/go-sqlite3 v1.14.22/go.mod h1:Uh1q+B4BYcTPb+yiD3kU8Ct7aC0hY9fxUwlHK0RXw+Y=
golang.org/x/crypto v0.21.0 h1:X31++rzVUdKhX5sWmSOFZxx8UW/ldWx55cbf08iNAMA=
golang.org/x/crypto v0.21.0/go.mod h1:0BP7YvVV9gBbVKyeTG0Gyn+gZm94bibOW5BjDEYAOMs=
```


Создаём файл **main.go**, в котором и будем экспериментировать

### Простой текст
Рассмотрим простую программу
```
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type User struct {
	ID       int64
	Name     string
	Password string
}

func (u User) ComparePassword(u2 User) error {
	if u.Password == u2.Password {
		log.Println("auth success")
		return nil
	}
	log.Println("auth fail")
	return fmt.Errorf("passwords don't match")
}

func createTable(ctx context.Context, db *sql.DB) error {
	const usersTable = `
	CREATE TABLE IF NOT EXISTS users(
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		name TEXT UNIQUE,
		password TEXT
	);`

	if _, err := db.ExecContext(ctx, usersTable); err != nil {
		return err
	}

	return nil
}

func insertUser(ctx context.Context, db *sql.DB, user *User) (int64, error) {
	var q = `
	INSERT INTO users (name, password) values ($1, $2)
	`
	result, err := db.ExecContext(ctx, q, user.Name, user.Password)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

func selectUser(ctx context.Context, db *sql.DB, name string) (User, error) {
	var (
		user User
		err  error
	)

	var q = "SELECT id, name, password FROM users WHERE name=$1"
	err = db.QueryRowContext(ctx, q, name).Scan(&user.ID, &user.Name, &user.Password)
	return user, err
}

func main() {
	ctx := context.TODO()

	db, err := sql.Open("sqlite3", "store.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	err = db.PingContext(ctx)
	if err != nil {
		panic(err)
	}

	if err = createTable(ctx, db); err != nil {
		panic(err)
	}

	user := &User{
		Name:     "Name",
		Password: "qwertyqwerty",
	}
	userID, err := insertUser(ctx, db, user)
	if err != nil {
		log.Println("user already exists")
	} else {
		user.ID = userID
	}

	userFromDB, err := selectUser(ctx, db, user.Name)
	if err != nil {
		panic(err)
	}

	user.ComparePassword(userFromDB)
	user.Password = "fail passsword"
	user.ComparePassword(userFromDB)
}
```
Здесь вам всё должно быть знакомо. Мы создали таблицу с пользователями. Обратите внимание, пароль хранится просто в виде тектса. Аутентификация пользователя производится простым сравненим строк.
Самая главная проблема с этим подходом - это небезопасно и неэтично. Кроме пользователя никто не должен знать пароль. В противном случе любой, кто может получить доступ к СУБД сможет притвориться таким пользователем. 
Если вам непонятен этот код - вернитесь к уроку **SQLite**


### Криптографические хеш-функции
Хеш-функция - это особый вид функции для сопоставления данных с некоторыми другими данными. Криптографические хеш-функции - это часть из них, специально разработанная так, чтобы хеширование было строго односторонним.    


Единственный способ аутентифицировать попытку входа в систему - это хэшировать входные учетные данные и сравнить хэш, хранящийся в нашей базе данных.

Наряду с каждым хешем есть случайная строка байтов, известная, как соль.

Напишем функцию генерации такого хэша
```
func generate(s string) (string, error) {
	saltedBytes := []byte(s)
	hashedBytes, err := bcrypt.GenerateFromPassword(saltedBytes, bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	hash := string(hashedBytes[:])
	return hash, nil
}
```
Тут мы пользуемся функцией **GenerateFromPassword** пакета **bcrypt**.

Напишем функцию сравнения хэша с паролем.
```
func compare(hash string, s string) error {
	incoming := []byte(s)
	existing := []byte(hash)
	return bcrypt.CompareHashAndPassword(existing, incoming)
}
```
Тут мы пользуемся функцией **CompareHashAndPassword** пакета **bcrypt**.

Модифицируем нашу программу таким образом, чтобы мы смогли работать с пользователями.

```
package main

import (
	"context"
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID             int64
	Name           string
	Password       string
	OriginPassword string
}

func (u User) ComparePassword(u2 User) error {
	err := compare(u2.Password, u.OriginPassword)
	if err != nil {
		log.Println("auth fail")
		return err
	}

	log.Println("auth success")
	return nil
}

func createTable(ctx context.Context, db *sql.DB) error {
	const usersTable = `
	CREATE TABLE IF NOT EXISTS users(
		id INTEGER PRIMARY KEY AUTOINCREMENT, 
		name TEXT UNIQUE,
		password TEXT
	);`

	if _, err := db.ExecContext(ctx, usersTable); err != nil {
		return err
	}

	return nil
}

func insertUser(ctx context.Context, db *sql.DB, user *User) (int64, error) {
	var q = `
	INSERT INTO users (name, password) values ($1, $2)
	`
	result, err := db.ExecContext(ctx, q, user.Name, user.Password)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

func selectUser(ctx context.Context, db *sql.DB, name string) (User, error) {
	var (
		user User
		err  error
	)

	var q = "SELECT id, name, password FROM users WHERE name=$1"
	err = db.QueryRowContext(ctx, q, name).Scan(&user.ID, &user.Name, &user.Password)
	return user, err
}

func generate(s string) (string, error) {
	saltedBytes := []byte(s)
	hashedBytes, err := bcrypt.GenerateFromPassword(saltedBytes, bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	hash := string(hashedBytes[:])
	return hash, nil
}

func compare(hash string, s string) error {
	incoming := []byte(s)
	existing := []byte(hash)
	return bcrypt.CompareHashAndPassword(existing, incoming)
}

func main() {
	ctx := context.TODO()

	db, err := sql.Open("sqlite3", "store.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	err = db.PingContext(ctx)
	if err != nil {
		panic(err)
	}

	if err = createTable(ctx, db); err != nil {
		panic(err)
	}

	password, err := generate("qwertyqwerty")
	if err != nil {
		panic(err)
	}

	user := &User{
		Name:           "New Name",
		Password:       password,
		OriginPassword: "qwertyqwerty",
	}
	userID, err := insertUser(ctx, db, user)
	if err != nil {
		log.Println("user already exists")
	} else {
		user.ID = userID
	}

	userFromDB, err := selectUser(ctx, db, user.Name)
	if err != nil {
		panic(err)
	}

	user.ComparePassword(userFromDB)
	user.Password, err = generate("fail passsword")
	if err != nil {
		panic(err)
	}
	user.OriginPassword = "fail passsword"
	user.ComparePassword(userFromDB)
}
```
Теперь мы вставляем в СУБД не пароль, а его хэш.

### Вместо заключения
Используйте приведенный пример в своем финальном проекте.
