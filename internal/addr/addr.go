// Package addr validates Kaspa addresses and builds BIP-21-style payment URIs.
package addr

import (
	"fmt"
	"net/url"
	"strings"
)

const charset = "qpzry9x8gf2tvdw0s3jn54khce6mua7l"

// Prefix returns the URI scheme for a Kaspa address, including the colon.
func Prefix(s string) string {
	s = strings.TrimSpace(s)
	switch {
	case strings.HasPrefix(s, "kaspatest:"):
		return "kaspatest:"
	case strings.HasPrefix(s, "kaspa:"):
		return "kaspa:"
	default:
		return ""
	}
}

// Testnet reports TN10 (kaspatest:) addresses.
func Testnet(s string) bool {
	return Prefix(s) == "kaspatest:"
}

// IndexerBase is the public REST host that can see this address.
func IndexerBase(s string) string {
	if Testnet(s) {
		return "https://api-tn10.kaspa.org"
	}
	return "https://api.kaspa.org"
}

// Valid reports whether s is a kaspa: or kaspatest: bech32 address.
func Valid(s string) bool {
	s = strings.TrimSpace(s)
	p := Prefix(s)
	if p == "" {
		return false
	}
	body := s[len(p):]
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
// Spec: rusty-kaspa issue #127 (BIP-21 analogue). kaspatest: is the TN10 form.
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
