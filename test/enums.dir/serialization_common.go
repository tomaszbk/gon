package main

import (
	"encoding/json"
	"fmt"
)

// This format belongs to this adapter, not to Gon's enum representation.
type wireEvent struct {
	Variant string `json:"variant"`
	Text    string `json:"text,omitempty"`
	Number  int    `json:"number,omitempty"`
	Ready   bool   `json:"ready,omitempty"`
}

func encodeEvent(event Event) ([]byte, error) { return json.Marshal(eventWire(event)) }

func decodeEvent(data []byte) (Event, error) {
	var wire wireEvent
	if err := json.Unmarshal(data, &wire); err != nil {
		var zero Event
		return zero, err
	}
	return eventFromWire(wire)
}

func main() {
	for _, event := range []Event{idleEvent(), textEvent(""), textEvent("hello"), countEvent(0, false), countEvent(7, true)} {
		data, err := encodeEvent(event)
		if err != nil {
			panic(err)
		}
		decoded, err := decodeEvent(data)
		if err != nil || decoded != event {
			panic(fmt.Sprintf("round trip: %s: %v", data, err))
		}
		fmt.Println(string(data))
	}
	for _, data := range []string{`{"variant":"future"}`, `{"variant":"count","number":"invalid"}`, `{`} {
		if _, err := decodeEvent([]byte(data)); err == nil {
			panic("invalid external value accepted")
		}
	}
	fmt.Println("explicit serialization: PASS")
}
