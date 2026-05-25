package envelope

import (
	"encoding/json"
	"fmt"
	"time"
)

const SpecVersion = "1.0"

type Event struct {
	ID              string
	Source          string
	Type            string
	Subject         string
	Time            time.Time
	DataContentType string
	Data            []byte
}

func (e Event) Validate() error {
	if e.ID == "" || e.Source == "" || e.Type == "" {
		return fmt.Errorf("cloudevents: id, source, and type are required")
	}
	return nil
}

func Encode(e Event) ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	if e.Time.IsZero() {
		e.Time = time.Now().UTC()
	}
	if e.DataContentType == "" {
		e.DataContentType = "application/json"
	}
	payload := map[string]any{
		"specversion":     SpecVersion,
		"id":              e.ID,
		"source":          e.Source,
		"type":            e.Type,
		"time":            e.Time.Format(time.RFC3339Nano),
		"datacontenttype": e.DataContentType,
		"data":            json.RawMessage(e.Data),
	}
	if e.Subject != "" {
		payload["subject"] = e.Subject
	}
	return json.Marshal(payload)
}

func Decode(b []byte) (Event, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return Event{}, err
	}
	e := Event{DataContentType: "application/json"}
	unquote := func(key string) string {
		v, ok := raw[key]
		if !ok {
			return ""
		}
		var s string
		_ = json.Unmarshal(v, &s)
		return s
	}
	e.ID = unquote("id")
	e.Source = unquote("source")
	e.Type = unquote("type")
	e.Subject = unquote("subject")
	e.DataContentType = unquote("datacontenttype")
	if t := unquote("time"); t != "" {
		parsed, err := time.Parse(time.RFC3339Nano, t)
		if err == nil {
			e.Time = parsed
		}
	}
	if d, ok := raw["data"]; ok {
		e.Data = append([]byte(nil), d...)
	}
	return e, e.Validate()
}
