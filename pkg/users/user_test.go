package users

import (
	"database/sql"
	"os"
	"testing"
)

type Test1 struct {
	In  string
	Out bool
}

func TestLoginValidation(t *testing.T) {
	tests := []Test1{
		{"Hi", false},
		{"Hello", true},
		{"MusicalTrickyPickySlang", false},
		{"Aristokrat", true},
		{"11123123", false},
		{"Music!Song?", false},
	}
	for _, test := range tests {
		got := LoginValidation(test.In)
		if got != test.Out {
			t.Fatalf("waited %t, but got %t", test.Out, got)
		}
	}
}

func TestHashingPassword(t *testing.T) {
	hash := HashingPassword("password")
	if len(hash) == 0 {
		t.Errorf("waited hash, got nothing")
	}
}

type Test2 struct {
	In      string
	Compare string
	Out     bool
}

func TestComparePasswords(t *testing.T) {
	tests := []Test2{
		{"Muha233", "Muha233", true},
		{"qwerty", "qwerty", true},
		{"qwerty123", "qwerty123", true},
		{"q1w2e3r4t5y6", "q1w2e3r4", false},
		{"ultraplus", "ultraplusikus", false},
	}
	for _, test := range tests {
		hash := HashingPassword(test.In)
		got := ComparePasswords(hash, test.Compare)
		if got != test.Out {
			t.Errorf("waited %t, got %t", test.Out, got)
		}
	}
}

func TestGetNewTokenJWT(t *testing.T) {
	token := GetNewTokenJWT("user")
	if len(token) == 0 {
		t.Errorf("waited token, got nothing")
	}
}

func TestValidateTokenJWT(t *testing.T) {
	db, _ := sql.Open("sqlite3", "data_test.db")
	defer db.Close()

	db.Exec("CREATE TABLE IF NOT EXISTS Users (login TEXT)")
	db.Exec("INSERT INTO Users (login) VALUES ('Muha')")

	token := GetNewTokenJWT("Muha")
	if !ValidateTokenJWT(token, "data_test.db") {
		t.Errorf("waited true, got false")
	}
	if ValidateTokenJWT("NotMuha", "data_test.db") {
		t.Errorf("waited false, got true")
	}

	db.Exec("DROP TABLE Users")
}

type Test3 struct {
	Login       string
	BadToken    bool
	Out         string
	WaitedError bool
}

func TestGetUserLogin(t *testing.T) {
	tests := []Test3{
		{Login: "Muha", BadToken: false, Out: "Muha", WaitedError: false},
		{Login: "MisterPickle", BadToken: true, Out: "MisterPickle", WaitedError: true},
	}
	for _, test := range tests {
		token := GetNewTokenJWT(test.Login)
		got := GetUserLogin(token)
		if test.BadToken {
			got = GetUserLogin("srryGuys")
		}
		if got != test.Out {
			if !test.WaitedError {
				t.Errorf("waited %s, got %s", test.Out, got)
			}
		}
	}
}

func TestCreateUserOperations(t *testing.T) {
	db, _ := sql.Open("sqlite3", "data_test.db")
	defer os.Remove("data_test.db")
	defer db.Close()
	db.Exec("CREATE TABLE IF NOT EXISTS Operations (operation_type TEXT, execution_time INT, user TEXT)")

	CreateUserOperations("Muha", "data_test.db")

	var count int
	db.QueryRow("SELECT COUNT(*) FROM Operations WHERE user = 'Muha'").Scan(&count)
	if count != 4 {
		t.Errorf("Expected 4, got %d", count)
	}

	db.Exec("DROP TABLE Operations")
}
