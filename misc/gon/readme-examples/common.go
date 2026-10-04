package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Config struct{ Mode string }
type User struct {
	Name        string
	Nickname    string
	HasNickname bool
}

var invalidConfig = errors.New("invalid config")

func parseConfig(data []byte) (Config, error) {
	if string(data) == "bad" {
		return Config{Mode: "partial"}, invalidConfig
	}
	return Config{Mode: string(data)}, nil
}

var events []string

func dimension(name string, value int) int {
	events = append(events, name)
	return value
}

func visit(name string, value int) int {
	events = append(events, name)
	return value
}

func formatList(title string, count int, unit string) string {
	return fmt.Sprintf("%s: %d %s", title, count, unit)
}

type partialReader struct{}

func (partialReader) Read(buffer []byte) (int, error) {
	return copy(buffer, "end"), io.EOF
}

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

func main() {
	directory, err := os.MkdirTemp("", "gon-readme-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, "config")
	if err := os.WriteFile(path, []byte("production"), 0600); err != nil {
		panic(err)
	}
	config, err := loadConfig(path)
	require(config.Mode == "production" && err == nil, "successful config")
	config, err = loadConfig(filepath.Join(directory, "missing"))
	require(config == (Config{}) && errors.Is(err, os.ErrNotExist), "wrapped read error")
	if err := os.WriteFile(path, []byte("bad"), 0600); err != nil {
		panic(err)
	}
	config, err = loadConfig(path)
	require(config == (Config{}) && errors.Is(err, invalidConfig), "propagation zeros")
	buffer := make([]byte, 8)
	n, err := (partialReader{}).Read(buffer)
	require(n == 3 && string(buffer[:n]) == "end" && errors.Is(err, io.EOF), "useful partial read")

	require(displayName(nil) == "guest", "nil user")
	require(displayName(&User{Name: "Ada"}) == "Ada", "present user")
	require(displayName(&User{}) == "", "empty name is present")
	require(greeting(User{}) == "guest", "absent nickname")
	require(greeting(User{Nickname: "Ada", HasNickname: true}) == "Ada", "present nickname")
	require(greeting(User{HasNickname: true}) == "", "empty nickname is present")
	require(optionNilPreserved(), "typed nil payload is present")
	require(optionPropagation(false) == "guest" && optionPropagation(true) == "Ada!", "Option boundary")

	var zero Payment
	require(describePayment(zero) == "pending", "enum zero")
	require(describePayment(newPaid("receipt-42")) == "paid: receipt-42", "record payload")
	require(describePayment(newRejected("declined")) == "rejected: declined", "positional payload")
	require(paymentStatus(42) == "paid: receipt-42", "Result success")
	require(paymentStatus(-1) == "failed: negative amount", "Result failure")
	require(resultNilPreserved(), "Err(nil) remains failure")

	users := []User{{Name: "Zoe"}, {Name: "Ada"}, {Name: "Ben"}}
	sortUsers(users)
	require(users[0].Name == "Ada" && users[1].Name == "Ben" && users[2].Name == "Zoe", "lambda callback")
	require(capturedValue() == 12, "lambda capture")
	require(itemLabel(1) == "1 item" && itemLabel(3) == "3 items", "conditional values")
	events = nil
	require(lazyValue(true) == 1 && strings.Join(events, ",") == "yes", "lazy true branch")
	events = nil
	require(lazyValue(false) == 2 && strings.Join(events, ",") == "no", "lazy false branch")
	events = nil
	require(previewSize() == "640x480" && strings.Join(events, ",") == "height,width", "named evaluation order")

	users = []User{{Name: "Zoe"}, {Name: "Ada"}, {Name: "Ben"}}
	require(userList(users, nil) == "guest: 3 users", "combined list nil owner and plural")
	require(users[0].Name == "Ada" && users[1].Name == "Ben" && users[2].Name == "Zoe", "combined list sorts users")
	require(userList(users[:1], &User{Name: "Sam"}) == "Sam: 1 user", "combined list present owner and singular")
	require(userList(nil, &User{}) == ": 0 users", "combined list empty title stays present")
	for _, scenario := range []struct{ input, want string }{
		{"", "8080"}, {"0", "0"}, {"443", "443"}, {"abc", "invalid port"},
	} {
		require(portLabel(scenario.input) == scenario.want, "optional port: "+scenario.input)
	}

	fmt.Println("PASS: errors, nil, Option, enums, matching, Result, lambdas, conditionals, named arguments")
}
