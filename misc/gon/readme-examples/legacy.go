package main

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
)

// BEGIN README error-context
func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	config, err := parseConfig(data)
	if err != nil {
		return Config{}, err
	}
	return config, nil
}

// END README error-context

// BEGIN README nil-default
func displayName(user *User) string {
	name := "guest"
	if user != nil {
		name = user.Name
	}
	return name
}

// END README nil-default

// BEGIN README option
func nickname(user User) (string, bool) {
	return user.Nickname, user.HasNickname
}

func greeting(user User) string {
	name, ok := nickname(user)
	if !ok {
		return "guest"
	}
	return name
}

// END README option

// BEGIN README alternatives
type paymentState int

const (
	pending paymentState = iota
	paid
	rejected
)

type Payment struct {
	state   paymentState
	Receipt string
	Reason  string
}

func describePayment(payment Payment) string {
	switch payment.state {
	case pending:
		return "pending"
	case paid:
		return "paid: " + payment.Receipt
	case rejected:
		return "rejected: " + payment.Reason
	default:
		panic("invalid payment state")
	}
}

// END README alternatives

// BEGIN README error-api
func charge(amount int) (Payment, error) {
	if amount < 0 {
		return Payment{}, errors.New("negative amount")
	}
	return newPaid(fmt.Sprintf("receipt-%d", amount)), nil
}

func paymentStatus(amount int) string {
	payment, err := charge(amount)
	if err != nil {
		return "failed: " + err.Error()
	}
	return describePayment(payment)
}

// END README error-api

// BEGIN README lambda
func sortUsers(users []User) {
	slices.SortFunc(users, func(a, b User) int {
		return cmp.Compare(a.Name, b.Name)
	})
}

// END README lambda

// BEGIN README conditional
func itemLabel(count int) string {
	label := "items"
	if count == 1 {
		label = "item"
	}
	return fmt.Sprintf("%d %s", count, label)
}

// END README conditional

// BEGIN README named-arguments
func previewSize() string {
	height := dimension("height", 480)
	width := dimension("width", 640)
	return resize(width, height)
}

// END README named-arguments

// BEGIN README user-list
func userList(users []User, owner *User) string {
	slices.SortFunc(users, func(a, b User) int {
		return cmp.Compare(a.Name, b.Name)
	})
	title := "guest"
	if owner != nil {
		title = owner.Name
	}
	unit := "users"
	if len(users) == 1 {
		unit = "user"
	}
	return formatList(title, len(users), unit)
}

// END README user-list

// BEGIN README optional-port
func parsePort(text string) (int, bool, error) {
	if text == "" {
		return 0, false, nil
	}
	port, err := strconv.Atoi(text)
	if err != nil {
		return 0, false, err
	}
	return port, true, nil
}

func portLabel(text string) string {
	port, found, err := parsePort(text)
	if err != nil {
		return "invalid port"
	}
	if !found {
		port = 8080
	}
	return strconv.Itoa(port)
}

// END README optional-port

func resize(width, height int) string   { return fmt.Sprintf("%dx%d", width, height) }
func newPaid(receipt string) Payment    { return Payment{state: paid, Receipt: receipt} }
func newRejected(reason string) Payment { return Payment{state: rejected, Reason: reason} }

func optionNilPreserved() bool {
	some := true
	var value *User
	fallbacks := 0
	if !some {
		fallbacks++
		value = &User{Name: "guest"}
	}
	return value == nil && fallbacks == 0
}

func optionPropagation(present bool) string {
	user := User{Nickname: "Ada", HasNickname: present}
	name, ok := nickname(user)
	if !ok {
		return "guest"
	}
	return name + "!"
}

func capturedValue() int {
	offset := 2
	var increment func(int) int = func(value int) int { return value + offset }
	offset = 5
	return increment(7)
}

func lazyValue(condition bool) int {
	if condition {
		return visit("yes", 1)
	}
	return visit("no", 2)
}

// BEGIN README additions
func userSummary(users []User) string {
	names := make([]string, 0, len(users))
	for _, user := range users {
		names = append(names, user.Name)
	}
	if len(names) > 0 && names[0] != "" {
		return fmt.Sprintf("%s: %d users", names[0], len(names))
	}
	return fmt.Sprintf("empty: %d users", len(names))
}

func settled(payment Payment) bool { return payment.state == paid }

// END README additions
