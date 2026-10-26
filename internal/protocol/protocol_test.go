package protocol

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeEncodeRoundTrip(t *testing.T) {
	line := []byte(`{"id":"1","method":"ping","params":{"n":5}}`)
	req, err := DecodeRequest(line)
	require.NoError(t, err)
	require.Equal(t, "1", req.ID)
	require.Equal(t, "ping", req.Method)

	resp := Response{ID: req.ID, Result: "pong"}
	b, err := Encode(resp)
	require.NoError(t, err)
	require.JSONEq(t, `{"id":"1","result":"pong"}`, string(b))
}

func TestDecodeRequestInvalid(t *testing.T) {
	_, err := DecodeRequest([]byte(`{not json`))
	require.Error(t, err)
}
