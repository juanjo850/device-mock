package protocol

import "encoding/json"

// Request is a single line-delimited JSON request.
type Request struct {
	ID     string          `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// ErrorObj describes a protocol-level error.
type ErrorObj struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Response is a single line-delimited JSON response.
type Response struct {
	ID     string    `json:"id,omitempty"`
	Result any       `json:"result,omitempty"`
	Error  *ErrorObj `json:"error,omitempty"`
}

// Encode marshals a response to a single line (no trailing newline).
func Encode(r Response) ([]byte, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// DecodeRequest parses a request from a line.
func DecodeRequest(line []byte) (Request, error) {
	var r Request
	if err := json.Unmarshal(line, &r); err != nil {
		return Request{}, err
	}
	return r, nil
}
