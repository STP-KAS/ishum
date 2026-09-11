package rate

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"
)

// Book holds the numbers used to lock an invoice.
// KasUSD comes from api.kaspa.org. EurUSD from Frankfurter (ECB).
type Book struct {
	KasUSD    float64   `json:"kasUsd"`
	EurUSD    float64   `json:"eurUsd"`
	FeeSompi  uint64    `json:"feeSompi"`
	FeeRate   uint64    `json:"feeRate"`
	FetchedAt time.Time `json:"fetchedAt"`
	Source    string    `json:"source"`
	Manual    bool      `json:"manual,omitempty"`
}

// TypicalMass is a plain P2PK transfer after Toccata (sompi/gram × mass).
const TypicalMass uint64 = 2036
const MinFeeRate uint64 = 100

var (
	mu   sync.Mutex
	last Book
)

func Last() Book {
	mu.Lock()
	defer mu.Unlock()
	return last
}

func SetManual(kasUSD, eurUSD float64) Book {
	mu.Lock()
	defer mu.Unlock()
	if kasUSD > 0 {
		last.KasUSD = kasUSD
	}
	if eurUSD > 0 {
		last.EurUSD = eurUSD
	}
	last.Manual = true
	last.FetchedAt = time.Now().UTC()
	last.Source = "merchant sign"
	return last
}

func Fetch(client *http.Client) (Book, error) {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	kas, err := fetchKas(client)
	if err != nil {
		mu.Lock()
		b := last
		mu.Unlock()
		if b.KasUSD > 0 {
			return b, nil
		}
		return Book{}, err
	}
	eur, err := fetchEUR(client)
	if err != nil {
		eur = 1.08
	}
	fr, mass := fetchFee(client)
	b := Book{
		KasUSD:    kas,
		EurUSD:    eur,
		FeeRate:   fr,
		FeeSompi:  fr * mass,
		FetchedAt: time.Now().UTC(),
		Source:    "api.kaspa.org + frankfurter.app",
	}
	mu.Lock()
	if last.Manual && last.KasUSD > 0 {
		b.KasUSD = last.KasUSD
		b.Manual = true
		b.Source = "merchant sign (kas) + " + b.Source
	}
	last = b
	mu.Unlock()
	return b, nil
}

func fetchKas(client *http.Client) (float64, error) {
	resp, err := client.Get("https://api.kaspa.org/info/price?string=false")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var wrap struct {
		Price float64 `json:"price"`
	}
	if json.Unmarshal(body, &wrap) != nil || wrap.Price <= 0 {
		return 0, fmt.Errorf("kas price")
	}
	return wrap.Price, nil
}

func fetchFee(client *http.Client) (rate, mass uint64) {
	mass = TypicalMass
	rate = MinFeeRate
	resp, err := client.Get("https://api.kaspa.org/info/fee-estimate")
	if err != nil {
		return rate, mass
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var wrap struct {
		PriorityBucket struct {
			Feerate int `json:"feerate"`
		} `json:"priorityBucket"`
	}
	if json.Unmarshal(body, &wrap) != nil {
		return rate, mass
	}
	if wrap.PriorityBucket.Feerate > 0 {
		rate = uint64(wrap.PriorityBucket.Feerate)
	}
	if rate < MinFeeRate {
		rate = MinFeeRate
	}
	return rate, mass
}

func fetchEUR(client *http.Client) (float64, error) {
	resp, err := client.Get("https://api.frankfurter.app/latest?from=EUR&to=USD")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var wrap struct {
		Rates map[string]float64 `json:"rates"`
	}
	if json.Unmarshal(body, &wrap) != nil || wrap.Rates["USD"] <= 0 {
		return 0, fmt.Errorf("eur usd")
	}
	return wrap.Rates["USD"], nil
}

// FiatToUSD converts amount in major units of currency into USD.
func FiatToUSD(amount float64, currency string, b Book) float64 {
	switch currency {
	case "EUR":
		return amount * b.EurUSD
	default:
		return amount
	}
}

// FiatToSompi converts a fiat major-unit amount into sompi at the locked book.
// Uniq is added (0–999 sompi) so two open invoices rarely share an exact due.
func FiatToSompi(amount float64, currency string, b Book, uniq uint64) uint64 {
	usd := FiatToUSD(amount, currency, b)
	if b.KasUSD <= 0 || usd <= 0 {
		return 0
	}
	kas := usd / b.KasUSD
	sompi := uint64(math.Round(kas * 100_000_000))
	if uniq > 0 {
		sompi += uniq % 1000
	}
	if sompi == 0 {
		sompi = 1
	}
	return sompi
}

// FiatToMicro is 6-decimal stables at 1.00 stable = 1.00 USD.
func FiatToMicro(amount float64, currency string, b Book) uint64 {
	usd := FiatToUSD(amount, currency, b)
	if usd <= 0 {
		return 0
	}
	return uint64(math.Round(usd * 1_000_000))
}

func FeeOrDefault(b Book) uint64 {
	if b.FeeSompi > 0 {
		return b.FeeSompi
	}
	return MinFeeRate * TypicalMass
}
