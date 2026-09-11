package addr

import "testing"

func TestURI(t *testing.T) {
	a := "kaspa:qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqkx9awp4e"
	if !Valid(a) {
		t.Fatal("valid")
	}
	u := URI(a, 250000000, "iabc")
	if u != a+"?amount=2.5&message=iabc" {
		t.Fatal(u)
	}
	if Valid("kaspa:nope") {
		t.Fatal("short")
	}
	if Valid("kaspatest:qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqkx9awp4e") {
		t.Fatal("testnet refused")
	}
}

func TestKasText(t *testing.T) {
	if KasText(1) != "0.00000001" {
		t.Fatal(KasText(1))
	}
	if MicroText(2_500_000) != "2.500000" {
		t.Fatal(MicroText(2_500_000))
	}
}
