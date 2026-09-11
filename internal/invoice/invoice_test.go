package invoice

import (
	"testing"
	"time"

	"ishum/internal/rate"
	"ishum/internal/storecfg"
)

func setup(t *testing.T) {
	t.Helper()
	if err := Init(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

func TestCreateAndExpire(t *testing.T) {
	setup(t)
	st := storecfg.Store{
		ID: "S1", PayTo: "kaspa:qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqkx9awp4e",
		Currency: "EUR", RequiredConf: 10, ExpiryMinutes: 15,
		EnableKAS: true, EnableKUSD: true, EnableUSDT: true,
	}
	inv, err := Create(NewReq{
		Amount: 2.5, Currency: "EUR", Item: "Coffee",
		Store: st, Book: rate.Book{KasUSD: 0.04, EurUSD: 1.1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inv.Status != New {
		t.Fatal(inv.Status)
	}
	if len(inv.Methods) != 3 {
		t.Fatalf("methods %d", len(inv.Methods))
	}
	kas := MethodOf(*inv, "kas")
	if kas == nil || kas.DueSompi == 0 || kas.URI == "" {
		t.Fatal(kas)
	}
	kusd := MethodOf(*inv, "kusd")
	if kusd.Live || kusd.DueMicro == 0 || kusd.Kind != "reserved-native" {
		t.Fatal(kusd)
	}
	usdt := MethodOf(*inv, "usdt")
	if !usdt.Guest || !usdt.Freeze || usdt.Issuer != "Tether" {
		t.Fatal(usdt)
	}
	if inv.Sequence != "kaspa-l1" || inv.FeeAsset != "KAS" || inv.FeeSompi == 0 {
		t.Fatalf("layers seq=%s fee=%s %d", inv.Sequence, inv.FeeAsset, inv.FeeSompi)
	}
}

func TestSeenSettles(t *testing.T) {
	setup(t)
	st := storecfg.Store{
		ID: "S1", PayTo: "kaspa:qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqkx9awp4e",
		RequiredConf: 10, ExpiryMinutes: 15, EnableKAS: true,
	}
	inv, err := Create(NewReq{
		Amount: 1, Currency: "USD", Item: "sale",
		Store: st, Book: rate.Book{KasUSD: 0.05, EurUSD: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	due := MethodOf(*inv, "kas").DueSompi
	got, err := Seen(inv.ID, "aa", due, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != Processing {
		t.Fatalf("want Processing got %s", got.Status)
	}
	got, err = Seen(inv.ID, "aa", due, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != Settled {
		t.Fatalf("want Settled got %s", got.Status)
	}
}

func TestUnderpayInvalidAfterExpiry(t *testing.T) {
	setup(t)
	st := storecfg.Store{
		ID: "S1", PayTo: "kaspa:qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqkx9awp4e",
		RequiredConf: 1, ExpiryMinutes: 15, EnableKAS: true,
	}
	inv, err := Create(NewReq{
		Amount: 1, Currency: "USD", Item: "sale",
		Store: st, Book: rate.Book{KasUSD: 0.05, EurUSD: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := SetExpires(inv.ID, time.Now().UTC().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	due := MethodOf(*inv, "kas").DueSompi
	got, err := Seen(inv.ID, "bb", due/2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != Invalid {
		t.Fatalf("want Invalid got %s", got.Status)
	}
}

func TestDemoStable(t *testing.T) {
	setup(t)
	st := storecfg.Store{
		ID: "S1", RequiredConf: 1, ExpiryMinutes: 15,
		EnableKUSD: true,
	}
	inv, err := Create(NewReq{
		Amount: 3, Currency: "USD", Item: "lunch",
		Store: st, Book: rate.Book{KasUSD: 0.05, EurUSD: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := DemoSettle(inv.ID, "kusd", false)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Demo || got.Status != Settled {
		t.Fatal(got)
	}
	_, err = DemoSettle(inv.ID, "kas", false)
	if err == nil {
		t.Fatal("kas should refuse without demo")
	}
	st.EnableKAS = true
	st.EnableKUSD = false
	inv2, err := Create(NewReq{
		Amount: 1, Currency: "USD", Item: "sale",
		Store: st, Book: rate.Book{KasUSD: 0.05, EurUSD: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = DemoSettle(inv2.ID, "kas", false)
	if err == nil {
		t.Fatal("live kas without ISHUM_DEMO")
	}
}

func TestQuotePartition(t *testing.T) {
	setup(t)
	st := storecfg.Store{
		ID: "S1", PayTo: "kaspa:qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqkx9awp4e",
		RequiredConf: 1, ExpiryMinutes: 15, EnableKAS: true,
	}
	a, err := Create(NewReq{Amount: 1, Currency: "USD", Store: st, Book: rate.Book{KasUSD: 0.05, EurUSD: 1}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Create(NewReq{Amount: 1, Currency: "USD", Store: st, Book: rate.Book{KasUSD: 0.05, EurUSD: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if a.QuoteID == "" || a.QuoteID != a.ID {
		t.Fatal(a.QuoteID)
	}
	if a.ID == b.ID {
		t.Fatal("same quote")
	}
	da := MethodOf(*a, "kas").DueSompi
	txid := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := Seen(a.ID, txid, da, 10); err != nil {
		t.Fatal(err)
	}
	if _, err := Seen(b.ID, txid, MethodOf(*b, "kas").DueSompi, 10); err == nil {
		t.Fatal("txid must bind one quote")
	}
	got, _ := Get(a.ID)
	if got.Status != Settled || got.Match != "amount" {
		t.Fatal(got.Status, got.Match)
	}
	got, _ = Get(b.ID)
	if got.Status != New {
		t.Fatal("b must stay open")
	}
}
