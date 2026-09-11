// Package addr validates Kaspa addresses and builds BIP-21-style payment URIs.
package addr

import (
	"fmt"
	"net/url"
	"strings"
)

const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

// Valid reports whether s is a mainnet kaspa: bech32 address.
func Valid(s string) bool {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "kaspa:") {
		return false
	}
	body := s[len("kaspa:"):]
	if len(body) < 50 || len(body) > 80 {
		return false
	}
	for i := 0; i < len(body); i++ {
		if strings.IndexByte(charset, body[i]) < 0 {
			return false
		}
	}
	return true
}

// URI is kaspa:<addr>?amount=<kas>&message=<invoice-id>
// Spec: rusty-kaspa issue #127 (BIP-21 analogue).
func URI(payTo string, sompi uint64, message string) string {
	payTo = strings.TrimSpace(payTo)
	if !Valid(payTo) {
		return ""
	}
	kas := fmt.Sprintf("%d.%08d", sompi/100_000_000, sompi%100_000_000)
	kas = strings.TrimRight(strings.TrimRight(kas, "0"), ".")
	u := payTo + "?amount=" + kas
	if message != "" {
		u += "&message=" + url.QueryEscape(message)
	}
	return u
}

func KasText(sompi uint64) string {
	return fmt.Sprintf("%d.%08d", sompi/100_000_000, sompi%100_000_000)
}

func MicroText(micro uint64) string {
	return fmt.Sprintf("%d.%06d", micro/1_000_000, micro%1_000_000)
}
