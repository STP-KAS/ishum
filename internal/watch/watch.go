// Package watch polls the public Kaspa REST indexer for inbound KAS.
// This is Ishum’s NBXplorer: watch-only, no keys.
package watch

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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
	Outputs                 []txOut `json:"outputs"`
}

func Poll(client *http.Client, virtualDAA uint64) {
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	if virtualDAA == 0 {
		virtualDAA = tip(client)
	}
	for _, inv := range invoice.OpenKAS() {
		if inv.PayTo == "" {
			continue
		}
		m := invoice.MethodOf(inv, "kas")
		if m == nil || m.DueSompi == 0 {
			continue
		}
		txs, err := addressTxs(client, inv.PayTo)
		if err != nil {
			continue
		}
		created := inv.Created.UnixMilli() - 3000
		for _, t := range txs {
			if t.BlockTime > 0 && t.BlockTime < created {
				continue
			}
			var paid uint64
			for _, o := range t.Outputs {
				if o.ScriptPublicKeyAddress == inv.PayTo {
					paid += o.Amount
				}
			}
			if paid < m.DueSompi {
				continue
			}
			// Unique-amount match. BTCPay uses a fresh address; we use unique sompi + time window.
			if paid > m.DueSompi+2000 {
				continue
			}
			conf := confirmations(t, virtualDAA)
			_, _ = invoice.Seen(inv.ID, t.TransactionID, paid, conf)
			break
		}
	}
}

func Loop(stop <-chan struct{}) {
	client := &http.Client{Timeout: 8 * time.Second}
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			Poll(client, tip(client))
		}
	}
}

func tip(client *http.Client) uint64 {
	resp, err := client.Get("https://api.kaspa.org/info/virtual-chain-blue-score")
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
	u := "https://api.kaspa.org/addresses/" + url.PathEscape(payTo) + "/full-transactions?limit=20&resolve_previous_outpoints=no"
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
	u := "https://api.kaspa.org/transactions/" + url.PathEscape(txid)
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
	var paid uint64
	for _, o := range t.Outputs {
		if o.ScriptPublicKeyAddress == inv.PayTo {
			paid += o.Amount
		}
	}
	if paid == 0 {
		return nil, fmt.Errorf("txid does not pay this store")
	}
	return invoice.Seen(inv.ID, t.TransactionID, paid, confirmations(t, tip(client)))
}
