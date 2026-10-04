package main

import "fmt"

type Event struct {
	kind   int
	text   string
	number int
	ready  bool
}

func idleEvent() Event                   { return Event{} }
func textEvent(text string) Event        { return Event{kind: 1, text: text} }
func countEvent(n int, ready bool) Event { return Event{kind: 2, number: n, ready: ready} }

func eventWire(event Event) wireEvent {
	switch event.kind {
	case 0:
		return wireEvent{Variant: "idle"}
	case 1:
		return wireEvent{Variant: "text", Text: event.text}
	case 2:
		return wireEvent{Variant: "count", Number: event.number, Ready: event.ready}
	}
	panic("invalid local tag")
}

func eventFromWire(wire wireEvent) (Event, error) {
	switch wire.Variant {
	case "idle":
		return idleEvent(), nil
	case "text":
		return textEvent(wire.Text), nil
	case "count":
		return countEvent(wire.Number, wire.Ready), nil
	default:
		return Event{}, fmt.Errorf("unknown variant %q", wire.Variant)
	}
}
