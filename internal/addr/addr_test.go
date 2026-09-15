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
	tn := "kaspatest:qzffl5xy9np46gkttyuftqnv2w04pr8g3wsp7c3vv8se3txtelx6q7c0v0ldx"
	if !Valid(tn) {
		t.Fatal("tn10 valid")
	}
	if !Testnet(tn) || IndexerBase(tn) != "https://api-tn10.kaspa.org" {
		t.Fatal("tn10 indexer")
	}
	u2 := URI(tn, 100000000, "iabc")
	if u2 != tn+"?amount=1&message=iabc" {
		t.Fatal(u2)
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
