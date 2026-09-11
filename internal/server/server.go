package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"ishum/internal/addr"
	"ishum/internal/invoice"
	"ishum/internal/rails"
	"ishum/internal/rate"
	"ishum/internal/storecfg"
	"ishum/internal/watch"
	"ishum/web"
)

type Server struct {
	Addr string
	T    *template.Template
	Demo bool
}

func New(listen, dataDir string) (*Server, error) {
	if err := storecfg.Init(dataDir); err != nil {
		return nil, err
	}
	if err := invoice.Init(dataDir); err != nil {
		return nil, err
	}
	t, err := template.New("").Funcs(template.FuncMap{
		"lower": strings.ToLower,
		"kas":   addr.KasText,
		"micro": addr.MicroText,
		"money": money,
	}).ParseFS(web.Templates, "templates/*.html")
	if err != nil {
		return nil, err
	}
	s := &Server{
		Addr: listen,
		T:    t,
		Demo: os.Getenv("ISHUM_DEMO") == "1",
	}
	invoice.Hook = s.hook
	go func() {
		_, _ = rate.Fetch(nil)
		for range time.Tick(60 * time.Second) {
			_, _ = rate.Fetch(nil)
		}
	}()
	return s, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	sub, err := fs.Sub(web.Static, "static")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
	mux.HandleFunc("/", s.home)
	mux.HandleFunc("/pos", s.pos)
	mux.HandleFunc("/pos/charge", s.charge)
	mux.HandleFunc("/pay/", s.pay)
	mux.HandleFunc("/receipt/", s.receipt)
	mux.HandleFunc("/invoices", s.invoices)
	mux.HandleFunc("/store", s.store)
	mux.HandleFunc("/button", s.button)
	mux.HandleFunc("/docs", s.docs)
	mux.HandleFunc("/qr/", s.qr)
	mux.HandleFunc("/api/v1/invoices", s.apiInvoices)
	mux.HandleFunc("/api/v1/invoices/", s.apiInvoice)
	mux.HandleFunc("/api/v1/rate", s.apiRate)
	mux.HandleFunc("/api/v1/store", s.apiStore)
	return mux
}

type page struct {
	Title    string
	Active   string
	Store    storecfg.Store
	Items    []storecfg.CatalogItem
	Invoice  *invoice.Invoice
	Invoices []invoice.Invoice
	Book     rate.Book
	Rails    []rails.Info
	Error    string
	Demo     bool
	Host     string
	Chosen   *invoice.Method
}

func (s *Server) view(w http.ResponseWriter, r *http.Request, name string, p page) {
	p.Store = storecfg.Get()
	p.Demo = s.Demo
	p.Rails = rails.All
	p.Book = rate.Last()
	p.Host = r.Host
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.T.ExecuteTemplate(w, name, p); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), 500)
	}
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	s.view(w, r, "home.html", page{Title: "Ishum", Active: "home"})
}

func (s *Server) pos(w http.ResponseWriter, r *http.Request) {
	s.view(w, r, "pos.html", page{
		Title: "Point of sale", Active: "pos", Items: storecfg.Items(),
	})
}

func (s *Server) charge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST", 405)
		return
	}
	_ = r.ParseForm()
	amount, _ := strconv.ParseFloat(strings.TrimSpace(r.Form.Get("amount")), 64)
	tip, _ := strconv.ParseFloat(strings.TrimSpace(r.Form.Get("tip")), 64)
	item := strings.TrimSpace(r.Form.Get("item"))
	st := storecfg.Get()
	book := rate.Last()
	if st.KasUSDOverride > 0 {
		book.KasUSD = st.KasUSDOverride
	}
	inv, err := invoice.Create(invoice.NewReq{
		Amount: amount, Currency: st.Currency, Item: item, Tip: tip,
		OrderID: "pos", Store: st, Book: book,
	})
	if err != nil {
		s.view(w, r, "pos.html", page{Title: "Point of sale", Active: "pos", Items: storecfg.Items(), Error: err.Error()})
		return
	}
	http.Redirect(w, r, "/pay/"+inv.ID, http.StatusSeeOther)
}

func (s *Server) pay(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/pay/")
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		rail := r.Form.Get("rail")
		if rail != "" {
			_, _ = invoice.Choose(id, rail)
		}
		if tx := strings.TrimSpace(r.Form.Get("txid")); tx != "" {
			if _, err := watch.Claim(nil, id, tx); err != nil {
				inv, _ := invoice.Get(id)
				s.view(w, r, "pay.html", page{Title: "Pay", Invoice: inv, Error: err.Error(), Chosen: chosen(inv, rail)})
				return
			}
		}
		if r.Form.Get("demo") == "1" {
			if _, err := invoice.DemoSettle(id, rail, s.Demo); err != nil {
				inv, _ := invoice.Get(id)
				s.view(w, r, "pay.html", page{Title: "Pay", Invoice: inv, Error: err.Error(), Chosen: chosen(inv, rail)})
				return
			}
		}
		http.Redirect(w, r, "/pay/"+id, http.StatusSeeOther)
		return
	}
	inv, ok := invoice.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if inv.Status == invoice.Settled {
		http.Redirect(w, r, "/receipt/"+id, http.StatusSeeOther)
		return
	}
	rail := r.URL.Query().Get("rail")
	if rail == "" {
		rail = inv.Chosen
	}
	s.view(w, r, "pay.html", page{Title: "Pay " + inv.ID, Invoice: inv, Chosen: chosen(inv, rail)})
}

func (s *Server) receipt(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/receipt/")
	inv, ok := invoice.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.view(w, r, "receipt.html", page{Title: "Receipt", Invoice: inv, Chosen: chosen(inv, inv.Chosen)})
}

func (s *Server) invoices(w http.ResponseWriter, r *http.Request) {
	s.view(w, r, "invoices.html", page{Title: "Invoices", Active: "invoices", Invoices: invoice.List(80)})
}

func (s *Server) store(w http.ResponseWriter, r *http.Request) {
	st := storecfg.Get()
	if r.Method == http.MethodPost {
		_ = r.ParseForm()
		st.Name = r.Form.Get("name")
		st.PayTo = strings.TrimSpace(r.Form.Get("payTo"))
		st.Currency = r.Form.Get("currency")
		st.Webhook = r.Form.Get("webhook")
		st.RequiredConf, _ = strconv.Atoi(r.Form.Get("requiredConfirmations"))
		st.ExpiryMinutes, _ = strconv.Atoi(r.Form.Get("expiryMinutes"))
		st.EnableKAS = r.Form.Get("enableKas") == "1"
		st.EnableKUSD = r.Form.Get("enableKusd") == "1"
		st.EnableUSDT = r.Form.Get("enableUsdt") == "1"
		if v := r.Form.Get("kasUsd"); v != "" {
			st.KasUSDOverride, _ = strconv.ParseFloat(v, 64)
			if st.KasUSDOverride > 0 {
				rate.SetManual(st.KasUSDOverride, 0)
			}
		}
		if st.PayTo != "" && !addr.Valid(st.PayTo) {
			s.view(w, r, "store.html", page{Title: "Store", Active: "store", Error: "pay-to must be a mainnet kaspa: address", Items: storecfg.Items()})
			return
		}
		if err := storecfg.Save(st, nil); err != nil {
			s.view(w, r, "store.html", page{Title: "Store", Active: "store", Error: err.Error(), Items: storecfg.Items()})
			return
		}
		http.Redirect(w, r, "/store", http.StatusSeeOther)
		return
	}
	s.view(w, r, "store.html", page{Title: "Store", Active: "store", Items: storecfg.Items()})
}

func (s *Server) button(w http.ResponseWriter, r *http.Request) {
	s.view(w, r, "button.html", page{Title: "Pay button", Active: "button"})
}

func (s *Server) docs(w http.ResponseWriter, r *http.Request) {
	s.view(w, r, "docs.html", page{Title: "API", Active: "docs"})
}

func (s *Server) qr(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/qr/")
	id = strings.TrimSuffix(id, ".png")
	inv, ok := invoice.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	rail := r.URL.Query().Get("rail")
	m := chosen(inv, rail)
	payload := ""
	if m != nil {
		payload = m.URI
		if payload == "" {
			payload = m.DueText + " " + inv.ID
		}
	}
	if payload == "" {
		payload = inv.ID
	}
	png, err := qrcode.Encode(payload, qrcode.Medium, 280)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(png)
}

func (s *Server) apiRate(w http.ResponseWriter, r *http.Request) {
	b, err := rate.Fetch(nil)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, b)
}

func (s *Server) apiStore(w http.ResponseWriter, r *http.Request) {
	if !s.auth(w, r) {
		return
	}
	writeJSON(w, 200, storecfg.Get())
}

func (s *Server) apiInvoices(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		if !s.auth(w, r) {
			return
		}
		writeJSON(w, 200, invoice.List(100))
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	if !s.auth(w, r) {
		return
	}
	var body struct {
		Amount   float64 `json:"amount"`
		Currency string  `json:"currency"`
		ItemDesc string  `json:"itemDesc"`
		OrderID  string  `json:"orderId"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeJSON(w, 400, map[string]string{"error": "json"})
		return
	}
	st := storecfg.Get()
	book := rate.Last()
	if st.KasUSDOverride > 0 {
		book.KasUSD = st.KasUSDOverride
	}
	inv, err := invoice.Create(invoice.NewReq{
		Amount: body.Amount, Currency: body.Currency, Item: body.ItemDesc,
		OrderID: body.OrderID, Store: st, Book: book,
	})
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, inv)
}

func (s *Server) apiInvoice(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/v1/invoices/")
	parts := strings.Split(rest, "/")
	id := parts[0]
	inv, ok := invoice.Get(id)
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "missing"})
		return
	}
	if len(parts) > 1 && parts[1] == "claim" && r.Method == http.MethodPost {
		if !s.auth(w, r) {
			return
		}
		var body struct {
			TxID string `json:"txid"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body)
		got, err := watch.Claim(nil, id, body.TxID)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": err.Error()})
			return
		}
		s.hook(got)
		writeJSON(w, 200, got)
		return
	}
	if r.Header.Get("Authorization") != "" && !s.auth(w, r) {
		return
	}
	writeJSON(w, 200, inv)
}

func (s *Server) auth(w http.ResponseWriter, r *http.Request) bool {
	st := storecfg.Get()
	got := r.Header.Get("Authorization")
	got = strings.TrimPrefix(got, "token ")
	got = strings.TrimPrefix(got, "Token ")
	if got == "" {
		got = r.URL.Query().Get("token")
	}
	if got == "" || got != st.APIKey {
		writeJSON(w, 401, map[string]string{"error": "token"})
		return false
	}
	return true
}

func (s *Server) hook(inv *invoice.Invoice) {
	if inv == nil {
		return
	}
	url := storecfg.Get().Webhook
	if url == "" {
		return
	}
	raw, _ := json.Marshal(inv)
	go func() {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Ishum-Event", "Invoice"+string(inv.Status))
		cli := &http.Client{Timeout: 8 * time.Second}
		resp, err := cli.Do(req)
		if err != nil {
			log.Println("webhook", err)
			return
		}
		resp.Body.Close()
	}()
}

func chosen(inv *invoice.Invoice, rail string) *invoice.Method {
	if inv == nil {
		return nil
	}
	if rail == "" {
		rail = inv.Chosen
	}
	if rail == "" && len(inv.Methods) > 0 {
		rail = inv.Methods[0].Rail
	}
	return invoice.MethodOf(*inv, rail)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func money(amount float64, ccy string) string {
	return fmt.Sprintf("%.2f %s", amount, ccy)
}
