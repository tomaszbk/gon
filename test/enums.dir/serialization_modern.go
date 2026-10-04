package main

import "fmt"

type Event enum {
	default Idle
	Text(string)
	Count  {
		Number int
		Ready  bool
	}
}

func idleEvent() Event                   { return Event.Idle }
func textEvent(text string) Event        { return Event.Text(text) }
func countEvent(n int, ready bool) Event { return Event.Count{Number: n, Ready: ready} }

func eventWire(event Event) wireEvent {
	return switch event {
	case Event.Idle => wireEvent{Variant: "idle"}
	case Event.Text(text) => wireEvent{Variant: "text", Text: text}
	case Event.Count{Number: n, Ready: ready} => wireEvent{Variant: "count", Number: n, Ready: ready}
	}
}

func eventFromWire(wire wireEvent) (Event, error) {
	switch wire.Variant {
	case "idle":
		return Event.Idle, nil
	case "text":
		return Event.Text(wire.Text), nil
	case "count":
		return Event.Count{Number: wire.Number, Ready: wire.Ready}, nil
	default:
		var zero Event
		return zero, fmt.Errorf("unknown variant %q", wire.Variant)
	}
}
