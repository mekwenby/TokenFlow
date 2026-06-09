package proxy

import (
	"bufio"
	"io"
	"strings"
)

type sseEvent struct {
	Event string
	Data  string
}

func readSSE(r io.Reader, emit func(sseEvent) error) error {
	reader := bufio.NewReader(r)
	var eventName string
	var data []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			if err == io.EOF && (eventName != "" || len(data) > 0) {
				return emit(sseEvent{Event: eventName, Data: strings.Join(data, "\n")})
			}
			if err == io.EOF {
				return nil
			}
			return err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if eventName != "" || len(data) > 0 {
				if err := emit(sseEvent{Event: eventName, Data: strings.Join(data, "\n")}); err != nil {
					return err
				}
			}
			eventName = ""
			data = nil
		} else if strings.HasPrefix(line, "event:") {
			eventName = strings.TrimSpace(line[len("event:"):])
		} else if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimSpace(line[len("data:"):]))
		}
		if err == io.EOF {
			return nil
		}
	}
}
