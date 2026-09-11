// Package invoice is BTCPay’s invoice engine, stripped to what a till needs.
//
// Statuses match Greenfield: New → Processing → Settled | Expired | Invalid.
// A Kaspa payment is Processing at 1 confirmation (already in a block).
// Settled when confirmations ≥ store policy (default 10 ≈ 10 seconds).
package invoice

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ishum/internal/addr"
	"ishum/internal/rails"
	"ishum/internal/rate"
	"ishum/internal/storecfg"
)

type Status string

const (
	New         Status = "New"
	Processing  Status = "Processing"
	Settled     Status = "Settled"
	Expired     Status = "Expired"
	Invalid     Status = "Invalid"
)

type Method struct {
	Rail     string `json:"rail"`
	Name     string `json:"name"`
	Unit     string `json:"unit"`
	Live     bool   `json:"live"`
	Kind     string `json:"kind"`
	Guest    bool   `json:"guest,omitempty"`
	Freeze   bool   `json:"freeze,omitempty"`
	Issuer   string `json:"issuer,omitempty"`
	Badge    string `json:"badge"`
	DueSompi uint64 `json:"dueSompi,omitempty"`
	DueMicro uint64 `json:"dueMicro,omitempty"`
	DueText  string `json:"dueText"`
	URI      string `json:"uri,omitempty"`
	Note     string `json:"note"`
}

type Invoice struct {
	ID            string    `json:"id"`
	StoreID       string    `json:"storeId"`
	OrderID       string    `json:"orderId,omitempty"`
	ItemDesc      string    `json:"itemDesc"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	Status        Status    `json:"status"`
	Created       time.Time `json:"created"`
	Expires       time.Time `json:"expires"`
	PaidAt        string    `json:"paidAt,omitempty"`
	RateKasUSD    float64   `json:"rateKasUsd"`
	RateEurUSD    float64   `json:"rateEurUsd"`
	PayTo         string    `json:"payTo"`
	Methods       []Method  `json:"methods"`
	Chosen        string    `json:"chosen,omitempty"`
	TxID          string    `json:"txid,omitempty"`
	Confirmations int       `json:"confirmations"`
	RequiredConf  int       `json:"requiredConfirmations"`
	PaidSompi     uint64    `json:"paidSompi,omitempty"`
	PaidMicro     uint64    `json:"paidMicro,omitempty"`
	Overpaid      bool      `json:"overpaid,omitempty"`
	Underpaid     bool      `json:"underpaid,omitempty"`
	Demo          bool      `json:"demo,omitempty"`
	Tip           float64   `json:"tip,omitempty"`
	Sequence      string    `json:"sequence"`
	FeeAsset      string    `json:"feeAsset"`
	FeeSompi      uint64    `json:"feeSompi"`
	FeeText       string    `json:"feeText"`
	QuoteID       string    `json:"quoteId"`
	Match         string    `json:"match,omitempty"`
}

type Event struct {
	At     time.Time `json:"at"`
	ID     string    `json:"id"`
	Type   string    `json:"type"`
	Status Status    `json:"status"`
	TxID   string    `json:"txid,omitempty"`
}

type index struct {
	Open  []string          `json:"open"`
	All   []string          `json:"all"`
	Taken map[string]string `json:"taken"`
	Events []Event          `json:"events"`
}

var (
	mu   sync.Mutex
	root string
	idx  index
	// Hook is called after a status change. Must not re-enter this package.
	Hook func(*Invoice)
)

func Init(dir string) error {
	root = dir
	if err := os.MkdirAll(invDir(), 0o755); err != nil {
		return err
	}
	idx = index{Taken: map[string]string{}}
	raw, err := os.ReadFile(idxPath())
	if err == nil {
		_ = json.Unmarshal(raw, &idx)
	}
	if idx.Taken == nil {
		idx.Taken = map[string]string{}
	}
	legacy := filepath.Join(dir, "invoices.json")
	if b, err := os.ReadFile(legacy); err == nil {
		var old struct {
			Invoices []Invoice `json:"invoices"`
			Events   []Event   `json:"events"`
		}
		if json.Unmarshal(b, &old) == nil {
			for _, inv := range old.Invoices {
				_ = writeInv(inv)
				idx.All = append(idx.All, inv.ID)
				if inv.Status == New || inv.Status == Processing {
					idx.Open = appendUnique(idx.Open, inv.ID)
				}
				if inv.TxID != "" {
					idx.Taken[inv.TxID] = inv.ID
				}
			}
			idx.Events = append(idx.Events, old.Events...)
			_ = saveIdx()
			_ = os.Rename(legacy, legacy+".migrated")
		}
	}
	return saveIdx()
}

func invDir() string  { return filepath.Join(root, "invoices") }
func idxPath() string { return filepath.Join(root, "index.json") }
func invPath(id string) string {
	return filepath.Join(invDir(), id+".json")
}

type NewReq struct {
	Amount   float64
	Currency string
	Item     string
	OrderID  string
	Tip      float64
	Store    storecfg.Store
	Book     rate.Book
}

func Create(r NewReq) (*Invoice, error) {
	if r.Amount <= 0 {
		return nil, fmt.Errorf("amount")
	}
	r.Currency = strings.ToUpper(strings.TrimSpace(r.Currency))
	if r.Currency == "" {
		r.Currency = r.Store.Currency
	}
	if r.Item == "" {
		r.Item = "sale"
	}
	id := "i" + hexid(6)
	uniq := fnv32(id)
	exp := r.Store.ExpiryMinutes
	if exp < 1 {
		exp = 15
	}
	conf := r.Store.RequiredConf
	if conf < 1 {
		conf = 10
	}
	fee := rate.FeeOrDefault(r.Book)
	inv := Invoice{
		ID:           id,
		StoreID:      r.Store.ID,
		OrderID:      r.OrderID,
		ItemDesc:     r.Item,
		Amount:       r.Amount + r.Tip,
		Currency:     r.Currency,
		Status:       New,
		Created:      time.Now().UTC(),
		Expires:      time.Now().UTC().Add(time.Duration(exp) * time.Minute),
		RateKasUSD:   r.Book.KasUSD,
		RateEurUSD:   r.Book.EurUSD,
		PayTo:        r.Store.PayTo,
		RequiredConf: conf,
		Tip:          r.Tip,
		Sequence:     rails.Sequence,
		FeeAsset:     rails.FeeAsset,
		FeeSompi:     fee,
		FeeText:      addr.KasText(fee) + " KAS",
		QuoteID:      id,
	}
	if r.Store.EnableKAS {
		due := rate.FiatToSompi(inv.Amount, inv.Currency, r.Book, uniq)
		inv.Methods = append(inv.Methods, settle(rails.KAS, due, 0, addr.URI(inv.PayTo, due, inv.ID)))
	}
	if r.Store.EnableKUSD {
		due := rate.FiatToMicro(inv.Amount, inv.Currency, r.Book)
		inv.Methods = append(inv.Methods, settle(rails.KUSD, 0, due, ""))
	}
	if r.Store.EnableUSDT {
		due := rate.FiatToMicro(inv.Amount, inv.Currency, r.Book)
		inv.Methods = append(inv.Methods, settle(rails.USDT, 0, due, ""))
	}
	if len(inv.Methods) == 0 {
		return nil, fmt.Errorf("no rails enabled")
	}
	mu.Lock()
	defer mu.Unlock()
	if err := writeInv(inv); err != nil {
		return nil, err
	}
	idx.All = append([]string{inv.ID}, idx.All...)
	if len(idx.All) > 500 {
		idx.All = idx.All[:500]
	}
	idx.Open = appendUnique(idx.Open, inv.ID)
	pushEventLocked(inv.ID, "InvoiceCreated", inv.Status, "")
	if err := saveIdx(); err != nil {
		return nil, err
	}
	cp := inv
	return &cp, nil
}

func Get(id string) (*Invoice, bool) {
	mu.Lock()
	defer mu.Unlock()
	inv, ok := readInv(id)
	if !ok {
		return nil, false
	}
	expireOne(&inv)
	cp := inv
	return &cp, true
}

func List(limit int) []Invoice {
	mu.Lock()
	defer mu.Unlock()
	ids := idx.All
	if limit <= 0 || limit > len(ids) {
		limit = len(ids)
	}
	out := make([]Invoice, 0, limit)
	for _, id := range ids {
		if len(out) >= limit {
			break
		}
		inv, ok := readInv(id)
		if !ok {
			continue
		}
		expireOne(&inv)
		out = append(out, inv)
	}
	return out
}

func OpenKAS() []Invoice {
	mu.Lock()
	defer mu.Unlock()
	var out []Invoice
	open := append([]string(nil), idx.Open...)
	for _, id := range open {
		inv, ok := readInv(id)
		if !ok {
			continue
		}
		expireOne(&inv)
		if inv.Status != New && inv.Status != Processing {
			continue
		}
		if method(inv, rails.KAS) == nil {
			continue
		}
		out = append(out, inv)
	}
	return out
}

func Choose(id, rail string) (*Invoice, error) {
	mu.Lock()
	defer mu.Unlock()
	inv, ok := readInv(id)
	if !ok {
		return nil, fmt.Errorf("missing")
	}
	expireOne(&inv)
	if inv.Status != New && inv.Status != Processing {
		return &inv, nil
	}
	if method(inv, rail) == nil {
		return nil, fmt.Errorf("rail")
	}
	inv.Chosen = rail
	if err := writeInv(inv); err != nil {
		return nil, err
	}
	return &inv, nil
}

// Seen records an on-chain KAS payment. conf is acceptances (Kaspa “confirmations”).
// One txid binds to one quote. That is the local double-spend rule, not a global mutex.
func Seen(id, txid string, paidSompi uint64, conf int) (*Invoice, error) {
	return seen(id, txid, paidSompi, conf, "amount")
}

func SeenPayload(id, txid string, paidSompi uint64, conf int) (*Invoice, error) {
	return seen(id, txid, paidSompi, conf, "payload")
}

func seen(id, txid string, paidSompi uint64, conf int, match string) (*Invoice, error) {
	mu.Lock()
	defer mu.Unlock()
	inv, ok := readInv(id)
	if !ok {
		return nil, fmt.Errorf("missing")
	}
	if inv.Status == Settled || inv.Status == Invalid {
		return &inv, nil
	}
	if other, used := idx.Taken[txid]; used && other != id {
		return nil, fmt.Errorf("txid already bound to %s", other)
	}
	m := method(inv, rails.KAS)
	if m == nil {
		return nil, fmt.Errorf("no kas method")
	}
	inv.TxID = txid
	inv.PaidSompi = paidSompi
	inv.Confirmations = conf
	inv.Chosen = rails.KAS
	inv.Match = match
	if paidSompi+1 < m.DueSompi { // 1 sompi slack
		inv.Underpaid = true
	}
	if paidSompi > m.DueSompi+1000 {
		inv.Overpaid = true
	}
	prev := inv.Status
	if inv.Underpaid && (inv.Status == Expired || time.Now().UTC().After(inv.Expires)) {
		inv.Status = Invalid
	} else if conf >= inv.RequiredConf && !inv.Underpaid {
		inv.Status = Settled
		if inv.PaidAt == "" {
			inv.PaidAt = time.Now().UTC().Format(time.RFC3339)
		}
	} else {
		inv.Status = Processing
		if inv.PaidAt == "" {
			inv.PaidAt = time.Now().UTC().Format(time.RFC3339)
		}
	}
	if prev != inv.Status {
		pushEventLocked(inv.ID, "Invoice"+string(inv.Status), inv.Status, txid)
	}
	if txid != "" {
		idx.Taken[txid] = inv.ID
	}
	if inv.Status != New && inv.Status != Processing {
		idx.Open = removeID(idx.Open, inv.ID)
	}
	if err := writeInv(inv); err != nil {
		return nil, err
	}
	if err := saveIdx(); err != nil {
		return nil, err
	}
	cp := inv
	if prev != inv.Status {
		fire(&cp)
	}
	return &cp, nil
}

// DemoSettle marks a reserved rail paid. KAS only if allowKAS (ISHUM_DEMO).
func DemoSettle(id, rail string, allowKAS bool) (*Invoice, error) {
	mu.Lock()
	defer mu.Unlock()
	inv, ok := readInv(id)
	if !ok {
		return nil, fmt.Errorf("missing")
	}
	m := method(inv, rail)
	if m == nil {
		return nil, fmt.Errorf("rail")
	}
	if m.Live && !allowKAS {
		return nil, fmt.Errorf("kas is live — send a real transaction or set ISHUM_DEMO=1")
	}
	inv.Chosen = rail
	inv.Demo = true
	inv.Status = Settled
	inv.Confirmations = inv.RequiredConf
	inv.PaidAt = time.Now().UTC().Format(time.RFC3339)
	inv.Match = "demo"
	if rail == rails.KAS {
		inv.PaidSompi = m.DueSompi
	} else {
		inv.PaidMicro = m.DueMicro
	}
	idx.Open = removeID(idx.Open, inv.ID)
	pushEventLocked(inv.ID, "InvoiceSettled", inv.Status, "demo")
	if err := writeInv(inv); err != nil {
		return nil, err
	}
	if err := saveIdx(); err != nil {
		return nil, err
	}
	fire(&inv)
	return &inv, nil
}

// SetExpires is for tests.
func SetExpires(id string, t time.Time) error {
	mu.Lock()
	defer mu.Unlock()
	inv, ok := readInv(id)
	if !ok {
		return fmt.Errorf("missing")
	}
	inv.Expires = t
	return writeInv(inv)
}

func Taken(txid string) (string, bool) {
	mu.Lock()
	defer mu.Unlock()
	id, ok := idx.Taken[txid]
	return id, ok
}

func Events(id string) []Event {
	mu.Lock()
	defer mu.Unlock()
	var out []Event
	for _, e := range idx.Events {
		if id == "" || e.ID == id {
			out = append(out, e)
		}
	}
	return out
}

func expireOne(inv *Invoice) {
	if inv.Status != New {
		return
	}
	if time.Now().UTC().After(inv.Expires) {
		inv.Status = Expired
		idx.Open = removeID(idx.Open, inv.ID)
		pushEventLocked(inv.ID, "InvoiceExpired", inv.Status, "")
		_ = writeInv(*inv)
		_ = saveIdx()
		cp := *inv
		fire(&cp)
	}
}

func method(inv Invoice, rail string) *Method {
	for i := range inv.Methods {
		if inv.Methods[i].Rail == rail {
			return &inv.Methods[i]
		}
	}
	return nil
}

func MethodOf(inv Invoice, rail string) *Method { return method(inv, rail) }

func settle(id string, sompi, micro uint64, uri string) Method {
	info, _ := rails.Get(id)
	m := Method{
		Rail: info.ID, Name: info.Name, Unit: info.Unit, Live: info.Live,
		Kind: info.Kind, Guest: info.Guest, Freeze: info.Freeze,
		Issuer: info.Issuer, Badge: info.Badge, Note: info.Note, URI: uri,
	}
	if sompi > 0 {
		m.DueSompi = sompi
		m.DueText = addr.KasText(sompi) + " KAS"
	}
	if micro > 0 {
		m.DueMicro = micro
		m.DueText = addr.MicroText(micro) + " " + info.Unit
	}
	return m
}

func pushEventLocked(id, typ string, st Status, tx string) {
	idx.Events = append([]Event{{
		At: time.Now().UTC(), ID: id, Type: typ, Status: st, TxID: tx,
	}}, idx.Events...)
	if len(idx.Events) > 2000 {
		idx.Events = idx.Events[:2000]
	}
}

func writeInv(inv Invoice) error {
	raw, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(invPath(inv.ID), raw, 0o644)
}

func readInv(id string) (Invoice, bool) {
	raw, err := os.ReadFile(invPath(id))
	if err != nil {
		return Invoice{}, false
	}
	var inv Invoice
	if json.Unmarshal(raw, &inv) != nil || inv.ID == "" {
		return Invoice{}, false
	}
	if inv.QuoteID == "" {
		inv.QuoteID = inv.ID
	}
	return inv, true
}

func saveIdx() error {
	raw, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(idxPath(), raw, 0o644)
}

func appendUnique(ids []string, id string) []string {
	for _, x := range ids {
		if x == id {
			return ids
		}
	}
	return append(ids, id)
}

func removeID(ids []string, id string) []string {
	out := make([]string, 0, len(ids))
	for _, x := range ids {
		if x != id {
			out = append(out, x)
		}
	}
	return out
}

func hexid(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func fnv32(s string) uint64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return uint64(h.Sum32())
}

func fire(inv *Invoice) {
	if Hook == nil || inv == nil {
		return
	}
	cp := *inv
	go Hook(&cp)
}
