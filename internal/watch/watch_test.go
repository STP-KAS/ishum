package watch

import (
	"encoding/hex"
	"testing"
)

func TestQuoteFromPayload(t *testing.T) {
	id := "ia1b2c3d4e5f6"
	if QuoteFromPayload(hex.EncodeToString([]byte(id))) != id {
		t.Fatal("hex utf8")
	}
	if QuoteFromPayload(id) != id {
		t.Fatal("plain")
	}
	if QuoteFromPayload("kaspa memo "+id+" extra") != id {
		t.Fatal("embedded")
	}
	if QuoteFromPayload("") != "" || QuoteFromPayload("null") != "" {
		t.Fatal("empty")
	}
}
