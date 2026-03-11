package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ValidateRequestFrame checks that required fields are present.
func ValidateRequestFrame(f *RequestFrame) error {
	if f.ID == "" {
		return errors.New("request frame: id is required")
	}
	if f.Method == "" {
		return errors.New("request frame: method is required")
	}
	return nil
}

// ValidateResponseFrame checks that required fields are consistent.
func ValidateResponseFrame(f *ResponseFrame) error {
	if f.ID == "" {
		return errors.New("response frame: id is required")
	}
	if f.OK && f.Payload == nil {
		return errors.New("response frame: payload is required when ok is true")
	}
	if !f.OK && f.Error == nil {
		return errors.New("response frame: error is required when ok is false")
	}
	return nil
}

// ValidateEventFrame checks that required fields are present.
func ValidateEventFrame(f *EventFrame) error {
	if f.Event == "" {
		return errors.New("event frame: event is required")
	}
	return nil
}

// ParseFrame unmarshals raw JSON into the appropriate frame type.
func ParseFrame(data []byte) (any, error) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("parse frame: %w", err)
	}

	switch envelope.Type {
	case FrameTypeReq:
		var f RequestFrame
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("parse request frame: %w", err)
		}
		if err := ValidateRequestFrame(&f); err != nil {
			return nil, err
		}
		return &f, nil

	case FrameTypeRes:
		var f ResponseFrame
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("parse response frame: %w", err)
		}
		if err := ValidateResponseFrame(&f); err != nil {
			return nil, err
		}
		return &f, nil

	case FrameTypeEvent:
		var f EventFrame
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("parse event frame: %w", err)
		}
		if err := ValidateEventFrame(&f); err != nil {
			return nil, err
		}
		return &f, nil

	default:
		return nil, fmt.Errorf("unknown frame type: %q", envelope.Type)
	}
}
