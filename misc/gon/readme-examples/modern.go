package main

import (
	"cmp"
	"fmt"
	"os"
	"slices"
	"strconv"
)

// BEGIN README error-context
func loadConfig(path string) (Config, error) {
	data := os.ReadFile(path) or err {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
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

// BEGIN README result
type ChargeResult = Result[Payment, string]

func charge(amount int) (payment ChargeResult) {
	if amount < 0 {
		return .Err("negative amount")
	}
	return .Ok(newPaid(fmt.Sprintf("receipt-%d", amount)))
}

func paymentStatus(amount int) string {
	payment := charge(amount) or reason {
		return "failed: " + reason
	}
	return describePayment(payment)
}

// END README result

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
func parsePort(text string) (port Result[int?, error]) {
	if text == "" {
		return .Ok(nil)
	}
	number := strconv.Atoi(text) or err {
		return .Err(err)
	}
	return .Ok(number)
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

func resultNilPreserved() bool {
	value := Result[int, error].Err(nil)
	return switch value {
	case Result[int, error].Ok(_) => false
	case Result[int, error].Err(problem) => problem == nil
	}
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
