package main

import (
	"cmp"
	"errors"
	"fmt"
	"gon/seq"
	"os"
	"slices"
	"strconv"
)

// BEGIN README error-context
func loadConfig(path string) (Config, error) {
	data := os.ReadFile(path) or err => fmt.Errorf("read config %q: %w", path, err)
	config := parseConfig(data)!
	return config, nil
}

// END README error-context

// BEGIN README nil-default
func displayName(user *User) string {
	return user?.Name ?? "guest"
}

// END README nil-default

// BEGIN README option
func nickname(user User) (name string?) {
	if !user.HasNickname {
		return nil
	}
	return user.Nickname
}

func greeting(user User) string {
	return nickname(user) ?? "guest"
}

// END README option

// BEGIN README alternatives
type Payment enum {
	default Pending
	Paid { Receipt string }
	Rejected(string)
}

func describePayment(payment Payment) string {
	return switch payment {
	case Payment.Pending => "pending"
	case Payment.Paid{Receipt: receipt} => "paid: " + receipt
	case Payment.Rejected(reason) => "rejected: " + reason
	}
}

// END README alternatives

// BEGIN README error-api
func charge(amount int) (Payment, error) {
	if amount < 0 {
		var zero Payment
		return zero, errors.New("negative amount")
	}
	return newPaid(fmt.Sprintf("receipt-%d", amount)), nil
}

func paymentStatus(amount int) string {
	payment := charge(amount) or err {
		return "failed: " + err.Error()
	}
	return describePayment(payment)
}

// END README error-api

// BEGIN README lambda
func sortUsers(users []User) {
	slices.SortFunc(users, (a, b) => cmp.Compare(a.Name, b.Name))
}

// END README lambda

// BEGIN README conditional
func itemLabel(count int) string {
	label := if count == 1 { "item" } else { "items" }
	return fmt.Sprintf("%d %s", count, label)
}

// END README conditional

// BEGIN README named-arguments
func previewSize() string {
	return resize(height: dimension("height", 480), width: dimension("width", 640))
}

// END README named-arguments

// BEGIN README user-list
func userList(users []User, owner *User) string {
	slices.SortFunc(users, (a, b) => cmp.Compare(a.Name, b.Name))
	title := owner?.Name ?? "guest"
	unit := if len(users) == 1 { "user" } else { "users" }
	return formatList(title: title, count: len(users), unit: unit)
}

// END README user-list

// BEGIN README optional-port
func parsePort(text string) (port int?, err error) {
	if text == "" {
		return nil, nil
	}
	number := strconv.Atoi(text)!
	return number, nil
}

func portLabel(text string) string {
	port := parsePort(text) or err {
		return "invalid port"
	}
	return strconv.Itoa(port ?? 8080)
}

// END README optional-port

func resize(width, height int) string   { return fmt.Sprintf("%dx%d", width, height) }
func newPaid(receipt string) Payment    { return Payment.Paid{Receipt: receipt} }
func newRejected(reason string) Payment { return Payment.Rejected(reason) }

func optionNilPreserved() bool {
	value := ((*User)?)((*User)(nil))
	fallbacks := 0
	fallback := func() *User { fallbacks++; return &User{Name: "guest"} }
	return (value ?? fallback()) == nil && fallbacks == 0
}

func excitedNickname(user User) string? {
	name := nickname(user)?
	return name + "!"
}

func optionPropagation(present bool) string {
	return excitedNickname(User{Nickname: "Ada", HasNickname: present}) ?? "guest"
}

func capturedValue() int {
	offset := 2
	var increment func(int) int = (value) => value + offset
	offset = 5
	return increment(7)
}

func lazyValue(condition bool) int {
	return if condition { visit("yes", 1) } else { visit("no", 2) }
}

// BEGIN README additions
func userSummary(users []User) string {
	names := seq.Map(users, (user) => user.Name)
	if seq.First(names) is first? && first != "" {
		return $"${first}: ${len(names)} users"
	}
	return $"empty: ${len(names)} users"
}

func settled(payment Payment) bool {
	return switch payment {
	case Payment.Pending, Payment.Rejected(_) => false
	case Payment.Paid{...} => true
	}
}

// END README additions
