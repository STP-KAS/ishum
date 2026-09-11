package rate

import "testing"

func TestFiatToSompi(t *testing.T) {
	b := Book{KasUSD: 0.04, EurUSD: 1.10}
	// 4.00 USD / 0.04 = 100 KAS = 10_000_000_000 sompi
	got := FiatToSompi(4, "USD", b, 0)
	if got != 10_000_000_000 {
		t.Fatalf("usd sompi %d", got)
	}
	// 4.00 EUR * 1.10 = 4.40 USD / 0.04 = 110 KAS
	got = FiatToSompi(4, "EUR", b, 0)
	if got != 11_000_000_000 {
		t.Fatalf("eur sompi %d", got)
	}
	micro := FiatToMicro(2.5, "USD", b)
	if micro != 2_500_000 {
		t.Fatalf("micro %d", micro)
	}
}

func TestUniq(t *testing.T) {
	b := Book{KasUSD: 0.04, EurUSD: 1}
	a := FiatToSompi(1, "USD", b, 42)
	c := FiatToSompi(1, "USD", b, 0)
	if a-c != 42 {
		t.Fatalf("%d %d", a, c)
	}
}
