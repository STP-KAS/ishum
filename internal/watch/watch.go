// Package watch polls the public Kaspa REST indexer for inbound KAS.
//
// One fetch per merchant address per tick. Matching is per quote (invoice id):
// payload first (Argent-style observe-by-id), unique sompi as fallback.
// A txid binds to one quote. Shop B is another address; it is not in this loop.
package watch

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"ishum/internal/addr"
	"ishum/internal/invoice"
)

type txOut struct {
	Amount                 uint64 `json:"amount"`
	ScriptPublicKeyAddress string `json:"script_public_key_address"`
}

type tx struct {
	TransactionID           string  `json:"transaction_id"`
	AcceptingBlockBlueScore uint64  `json:"accepting_block_blue_score"`
	IsAccepted              bool    `json:"is_accepted"`
	BlockTime               int64   `json:"block_time"`
	Payload                 string  `json:"payload"`
	Outputs                 []txOut `json:"outputs"`
}

func Poll(client *http.Client, virtualDAA uint64) {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	if virtualDAA == 0 {
		virtualDAA = tip(client)
	}
	byAddr := map[string][]invoice.Invoice{}
	for _, inv := range invoice.OpenKAS() {
		if inv.PayTo == "" {
			continue
		}
		byAddr[inv.PayTo] = append(byAddr[inv.PayTo], inv)
	}
	for payTo, invs := range byAddr {
		txs, err := addressTxs(client, payTo)
		if err != nil {
			continue
		}
		daa := virtualDAA
		if addr.Testnet(payTo) {
			daa = tipAt(client, addr.IndexerBase(payTo))
		}
		matchAddress(payTo, invs, txs, daa)
	}
}

func matchAddress(payTo string, invs []invoice.Invoice, txs []tx, virtualDAA uint64) {
	byQuote := map[string]invoice.Invoice{}
	for _, inv := range invs {
		byQuote[inv.QuoteID] = inv
		if inv.QuoteID != inv.ID {
			byQuote[inv.ID] = inv
		}
	}
	for _, t := range txs {
		if _, taken := invoice.Taken(t.TransactionID); taken {
			continue
		}
		paid := paidTo(t, payTo)
		if paid == 0 {
			continue
		}
		conf := confirmations(t, virtualDAA)
		if qid := QuoteFromPayload(t.Payload); qid != "" {
			if inv, ok := byQuote[qid]; ok {
				_, _ = invoice.SeenPayload(inv.ID, t.TransactionID, paid, conf)
				continue
			}
		}
		for _, inv := range invs {
			if _, taken := invoice.Taken(t.TransactionID); taken {
				break
			}
			m := invoice.MethodOf(inv, "kas")
			if m == nil || m.DueSompi == 0 {
				continue
			}
			if t.BlockTime > 0 && t.BlockTime < inv.Created.UnixMilli()-3000 {
				continue
			}
			if paid < m.DueSompi || paid > m.DueSompi+2000 {
				continue
			}
			_, _ = invoice.Seen(inv.ID, t.TransactionID, paid, conf)
			break
		}
	}
}

func paidTo(t tx, payTo string) uint64 {
	var paid uint64
	for _, o := range t.Outputs {
		if o.ScriptPublicKeyAddress == payTo {
			paid += o.Amount
		}
	}
	return paid
}

func Loop(stop <-chan struct{}) {
	client := &http.Client{Timeout: 8 * time.Second}
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			Poll(client, tip(client))
		}
	}
}

func tip(client *http.Client) uint64 {
	return tipAt(client, "https://api.kaspa.org")
}

func tipAt(client *http.Client, base string) uint64 {
	resp, err := client.Get(base + "/info/virtual-chain-blue-score")
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	var wrap struct {
		BlueScore uint64 `json:"blueScore"`
	}
	if json.Unmarshal(body, &wrap) != nil {
		return 0
	}
	return wrap.BlueScore
}

func addressTxs(client *http.Client, payTo string) ([]tx, error) {
	u := addr.IndexerBase(payTo) + "/addresses/" + url.PathEscape(payTo) + "/full-transactions?limit=20&resolve_previous_outpoints=no"
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("indexer %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var txs []tx
	if err := json.Unmarshal(body, &txs); err != nil {
		return nil, err
	}
	return txs, nil
}

func ParseSompi(s string) uint64 {
	s = strings.TrimSpace(s)
	n, _ := strconv.ParseUint(s, 10, 64)
	return n
}

func confirmations(t tx, virtualDAA uint64) int {
	if !t.IsAccepted {
		return 0
	}
	if virtualDAA > 0 && t.AcceptingBlockBlueScore > 0 && virtualDAA >= t.AcceptingBlockBlueScore {
		c := int(virtualDAA - t.AcceptingBlockBlueScore)
		if c < 1 {
			return 1
		}
		return c
	}
	return 1
}

// QuoteFromPayload reads an invoice/quote id out of a Kaspa tx payload.
// Wallets that copy the kaspa: URI message field land here. That is observe-by-id.
func QuoteFromPayload(payload string) string {
	payload = strings.TrimSpace(payload)
	if payload == "" || payload == "null" {
		return ""
	}
	raw := []byte(payload)
	if b, err := hex.DecodeString(strings.TrimPrefix(payload, "0x")); err == nil {
		raw = b
	}
	s := payload
	if utf8.Valid(raw) {
		s = string(raw)
	}
	s = strings.ToLower(s)
	i := strings.IndexByte(s, 'i')
	for i >= 0 {
		if i+13 <= len(s) {
			id := s[i : i+13]
			if id[0] == 'i' && isHex(id[1:]) {
				return id
			}
		}
		next := strings.IndexByte(s[i+1:], 'i')
		if next < 0 {
			break
		}
		i = i + 1 + next
	}
	return ""
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return len(s) == 12
}

// Claim binds a pasted txid to an open invoice if it pays the merchant.
func Claim(client *http.Client, id, txid string) (*invoice.Invoice, error) {
	inv, ok := invoice.Get(id)
	if !ok {
		return nil, fmt.Errorf("missing")
	}
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	txid = strings.TrimSpace(txid)
	if len(txid) != 64 {
		return nil, fmt.Errorf("txid")
	}
	base := addr.IndexerBase(inv.PayTo)
	u := base + "/transactions/" + url.PathEscape(txid)
	resp, err := client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("txid not found")
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var t tx
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, err
	}
	paid := paidTo(t, inv.PayTo)
	if paid == 0 {
		return nil, fmt.Errorf("txid does not pay this store")
	}
	if qid := QuoteFromPayload(t.Payload); qid != "" && qid != inv.ID && qid != inv.QuoteID {
		return nil, fmt.Errorf("txid payload is quote %s", qid)
	}
	daa := tipAt(client, base)
	if QuoteFromPayload(t.Payload) != "" {
		return invoice.SeenPayload(inv.ID, t.TransactionID, paid, confirmations(t, daa))
	}
	return invoice.Seen(inv.ID, t.TransactionID, paid, confirmations(t, daa))
}
