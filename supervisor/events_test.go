package supervisor

import (
	"bufio"
	"io"
	"strconv"
	"testing"
)

// Compare two string/string maps.
func cmpMap(m1 map[string]string, m2 map[string]string) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v := range m2 {
		if v != m1[k] {
			return false
		}
	}
	return true
}

// Compare two byte arrays.
func cmpBytes(p1 []byte, p2 []byte) bool {
	if p1 == nil {
		p1 = []byte{}
	}
	if p2 == nil {
		p2 = []byte{}
	}

	if len(p1) != len(p2) {
		return false
	}
	for i, v := range p2 {
		if v != p1[i] {
			return false
		}
	}
	return true
}

// Compare two events
func cmpEvents(e1 *Event, e2 *Event) bool {
	switch {
	case e1 == nil && e2 == nil:
		return true
	case e1 == nil || e2 == nil:
		return false
	default:
		return cmpMap(e1.Header, e2.Header) && cmpMap(e1.Meta, e2.Meta) && cmpBytes(e1.Payload, e2.Payload)
	}
}

// Construct an event.
func createEvent(serial int, eventname string, processname string, payload []byte) *Event {
	serialstr := strconv.Itoa(serial)
	return &Event{
		map[string]string{
			"ver":        "3.0",
			"server":     "supervisor",
			"eventname":  eventname,
			"serial":     serialstr,
			"pool":       "listener",
			"poolserial": serialstr,
		},
		map[string]string{
			"processname": processname,
			"groupname":   processname,
		},
		payload,
	}
}

// Test the ReadEvent function.
func TestRead(t *testing.T) {
	reader, writer := io.Pipe()
	bufReader := bufio.NewReader(reader)
	serial := 0

	sendAndVerify := func(eventname string, payload []byte) {
		sentEvent := createEvent(serial, eventname, "test", payload)
		serial++

		go func() {
			_, err := writer.Write(sentEvent.ToBytes())
			if err != nil {
				t.Error(err)
			}
		}()

		receiveEvent, err := ReadEvent(bufReader)
		if err != nil {
			t.Error(err)
		}

		if !cmpEvents(sentEvent, receiveEvent) {
			t.Error("invalid event received")
		}
	}

	sendAndVerify("EVENT_EMPTY_PAYLOAD", []byte{})
	sendAndVerify("EVENT_FULL_PAYLOAD", []byte("this is a payload test"))
}

// Test the WriteResult functions.
func TestWrite(t *testing.T) {
	reader, writer := io.Pipe()

	readAndVerify := func(expected string) {
		payload, err := ReadResult(reader)
		switch {
		case err != nil:
			t.Error(err)
		case string(payload) != expected:
			t.Errorf("Payload result invalid: %s != %s", payload, expected)
		}
	}

	payload := "some arbitrary data"
	go WriteResult(writer, []byte(payload))
	readAndVerify(payload)

	go WriteResultOK(writer)
	readAndVerify("OK")

	go WriteResultFail(writer)
	readAndVerify("FAIL")
}

// Test the Listen function.
func TestListen(t *testing.T) {
	stdin, stdinWriter := io.Pipe()
	stdoutReader, stdout := io.Pipe()

	ch := make(chan *Event, 1)
	reader := bufio.NewReader(stdoutReader)

	go func() {
		if err := Listen(stdin, stdout, ch); err != nil {
			t.Error(err)
		}
	}()

	serial := 0
	sendAndVerify := func(eventname string, payload []byte) {
		sentEvent := createEvent(serial, eventname, "test", payload)
		serial++

		bytes := sentEvent.ToBytes()
		_, err := stdinWriter.Write(bytes)
		if err != nil {
			t.Error(err)
		}

		result, err := ReadResult(reader)
		if err != nil {
			t.Error(err)
		}
		if string(result) != "OK" {
			t.Error("invalid result")
		}

		receiveEvent, ok := <-ch
		if !ok {
			t.Error("channel closed")
		} else if !cmpEvents(sentEvent, receiveEvent) {
			t.Error("invalid event received")
		}
	}

	sendAndVerify("PROCESS_STATE_RUNNING", []byte{})
	sendAndVerify("PROCESS_LOG_STDERR", []byte("some pretend log data"))
}
