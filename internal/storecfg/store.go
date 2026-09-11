package storecfg

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Store struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	PayTo          string  `json:"payTo"`
	Currency       string  `json:"currency"`
	RequiredConf   int     `json:"requiredConfirmations"`
	ExpiryMinutes  int     `json:"expiryMinutes"`
	Webhook        string  `json:"webhook,omitempty"`
	APIKey         string  `json:"apiKey"`
	EnableKAS      bool    `json:"enableKas"`
	EnableKUSD     bool    `json:"enableKusd"`
	EnableUSDT     bool    `json:"enableUsdt"`
	KasUSDOverride float64 `json:"kasUsdOverride,omitempty"`
}

type CatalogItem struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	Price float64 `json:"price"`
	Emoji string  `json:"emoji,omitempty"`
}

var DefaultItems = []CatalogItem{
	{ID: "coffee", Title: "Coffee", Price: 2.50, Emoji: "☕"},
	{ID: "tea", Title: "Tea", Price: 2.00, Emoji: "🍵"},
	{ID: "pastry", Title: "Pastry", Price: 3.20, Emoji: "🥐"},
	{ID: "lunch", Title: "Lunch", Price: 9.50, Emoji: "🍲"},
}

type file struct {
	Store Store         `json:"store"`
	Items []CatalogItem `json:"items"`
}

var (
	mu   sync.Mutex
	path string
	live file
)

func Init(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path = filepath.Join(dir, "store.json")
	b, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(b, &live)
	}
	if live.Store.ID == "" {
		live.Store.ID = "S" + hexid(4)
	}
	if live.Store.Name == "" {
		live.Store.Name = "Ishum counter"
	}
	if live.Store.Currency == "" {
		live.Store.Currency = "EUR"
	}
	if live.Store.RequiredConf == 0 {
		live.Store.RequiredConf = 10
	}
	if live.Store.ExpiryMinutes == 0 {
		live.Store.ExpiryMinutes = 15
	}
	if live.Store.APIKey == "" {
		live.Store.APIKey = hexid(16)
	}
	if !live.Store.EnableKAS && !live.Store.EnableKUSD && !live.Store.EnableUSDT {
		live.Store.EnableKAS = true
		live.Store.EnableKUSD = true
		live.Store.EnableUSDT = true
	}
	if len(live.Items) == 0 {
		live.Items = append([]CatalogItem(nil), DefaultItems...)
	}
	return saveLocked()
}

func Get() Store {
	mu.Lock()
	defer mu.Unlock()
	return live.Store
}

func Items() []CatalogItem {
	mu.Lock()
	defer mu.Unlock()
	out := make([]CatalogItem, len(live.Items))
	copy(out, live.Items)
	return out
}

func Save(s Store, items []CatalogItem) error {
	mu.Lock()
	defer mu.Unlock()
	if s.ID == "" {
		s.ID = live.Store.ID
	}
	if s.APIKey == "" {
		s.APIKey = live.Store.APIKey
	}
	s.Currency = strings.ToUpper(strings.TrimSpace(s.Currency))
	if s.Currency != "EUR" && s.Currency != "USD" {
		s.Currency = "EUR"
	}
	if s.RequiredConf < 1 {
		s.RequiredConf = 1
	}
	if s.ExpiryMinutes < 1 {
		s.ExpiryMinutes = 15
	}
	s.Name = strings.TrimSpace(s.Name)
	s.PayTo = strings.TrimSpace(s.PayTo)
	s.Webhook = strings.TrimSpace(s.Webhook)
	live.Store = s
	if len(items) > 0 {
		live.Items = items
	}
	return saveLocked()
}

func saveLocked() error {
	raw, err := json.MarshalIndent(live, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func hexid(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
