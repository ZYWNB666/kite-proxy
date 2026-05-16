package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrUnauthorized     = errors.New("unauthorized")
	ErrProxyForbidden   = errors.New("proxy forbidden")
	ErrNamespaceBlocked = errors.New("namespace forbidden")
)

// RequestError represents a non-2xx response returned by kite.
type RequestError struct {
	Err        error
	StatusCode int
	Code       string
	Message    string
}

func (e *RequestError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("HTTP %d", e.StatusCode)
}

func (e *RequestError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *RequestError) Is(target error) bool {
	if e == nil {
		return false
	}
	return errors.Is(e.Err, target)
}

type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func parseHTTPError(statusCode int, body []byte) error {
	message := strings.TrimSpace(string(body))
	code := ""

	var payload errorResponse
	if err := json.Unmarshal(body, &payload); err == nil {
		if payload.Error != "" {
			message = payload.Error
		}
		code = payload.Code
	}

	switch statusCode {
	case 401:
		if message == "" {
			message = "Invalid API key"
		}
		return &RequestError{
			Err:        ErrUnauthorized,
			StatusCode: statusCode,
			Code:       firstNonEmpty(code, "unauthorized"),
			Message:    message,
		}
	case 403:
		if message == "" {
			message = "no clusters available for proxy or proxy not permitted"
		}
		return &RequestError{
			Err:        ErrProxyForbidden,
			StatusCode: statusCode,
			Code:       firstNonEmpty(code, "proxy_forbidden"),
			Message:    message,
		}
	default:
		if message == "" {
			message = fmt.Sprintf("kite server returned %d", statusCode)
		}
		return &RequestError{
			StatusCode: statusCode,
			Code:       code,
			Message:    message,
		}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func ErrorCode(err error) string {
	var reqErr *RequestError
	if errors.As(err, &reqErr) {
		return reqErr.Code
	}
	return ""
}

func ErrorStatus(err error) int {
	var reqErr *RequestError
	if errors.As(err, &reqErr) {
		return reqErr.StatusCode
	}
	return 0
}
