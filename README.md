# Ishum

**sequenced on Kaspa**

Self-hosted payment processor. Invoice, point of sale, pay button, webhook. Funds go to the Kaspa address you set. This process never holds keys.

Four layers, not one coin:

| Layer | In Ishum |
| --- | --- |
| Consensus / order | Kaspa L1. USDT does not vote in GHOSTDAG. |
| Miner fee | Always KAS. Sender pays. Not Tether-as-gas. |
| Unit of account | EUR or USD on the keypad. |
| Settlement | KAS (native, live) · kUSD (reserved; BitCoffee protocol exists on TN10, this till cannot yet transfer the Asset ID) · USDT (guest IOU, freeze) |

USDT in does not un-decentralize Kaspa. It imports a king into the money people use. The till labels that. If USDT were the fee asset, miners would sit under a freeze-capable issuer. Refused.

“Sequenced on Kaspa” is the related sub-series of **this invoice** (create → pay → settle, no double-spend). It is not a global DeFi mutex. Sutton ([11 Sep 2026](https://x.com/michaelsuttonil/status/2098204180406026482)): reality updates in parallel; force partitioned app state. Argent DEX: Core is rare; each pair has `quote_id`; swaps do not contend on Core.

Ishum maps that: store ≈ Core, invoice ≈ pair/quote. One file per invoice. Watch fetches each pay-to once, binds a txid to one quote (payload/`message` first, unique sompi fallback). Two shops do not share a loop. A shared USDT/kUSD pool would. Refused.

Not solved here: unique HD address per invoice (needs xpub). Payload match needs the wallet to copy the URI message into the tx payload.

```
git clone https://github.com/STP-KAS/ishum.git
cd ishum
go test ./...
go build -o ishum.exe ./cmd/ishum
./ishum.exe
```

http://127.0.0.1:8090/pos

Set the receive address under `/store`. `kaspa:` (mainnet) and `kaspatest:` (TN10) are valid. Without it, KAS invoices cannot be paid; the two stable rails still demonstrate checkout. TN10 addresses are watched on `api-tn10.kaspa.org`.

## Why not a BTCPay plugin

BTCPay is the right *shape*. A Kaspa *plugin* is the documented altcoin path (C# handler + docker daemon, same as Monero). It is more work than this PoC and would not run here: no BTCPay stack, Kaspa is not Bitcoin-script, no issued Kaspa stable. Mapping: `plugin-btcpay/README.md`.

## Rails

| ID | Kind | Live | What it is |
| --- | --- | --- | --- |
| `kas` | native | yes | No issuer. Watch `api.kaspa.org` or `api-tn10.kaspa.org` for `kaspatest:`. Paste txid as fallback. |
| `kusd` | reserved-native | no | BitCoffee KUSD is a TN10 covenant protocol. This till still cannot move that Asset ID. Not Tether. No free dollar. |
| `usdt` | guest-iou | no | Tether prints, burns, freezes. Chain is still P2P. The unit is Tether policy. Not gas. |

KAS rate: `https://api.kaspa.org/info/price`. EUR: Frankfurter/ECB. Merchant can override KAS/USD on `/store`.

## Invoice states

New → Processing (seen, 1+ acceptance) → Settled (N acceptances, default 10).

Unpaid past expiry → Expired. Underpaid past expiry → Invalid.

Bitcoin POS uses Lightning because 10 minutes is not a counter. Kaspa L1 is ~1s blocks; 10 confirmations is ~10 seconds.

Matching for KAS: unique sompi (invoice id salt 0–999) + time window. BTCPay uses a fresh HD address instead. Paste-txid claims any payment to the store address.

## POS

`/pos` — keypad (BTCPay Light) or cart (BTCPay Cart). Tip 0/10/15/20%. Charge opens `/pay/{id}` with three rails and a QR (`kaspa:` URI).

## API

`Authorization: token <key>` (printed on `/store`).

```
POST /api/v1/invoices   {"amount":2.5,"currency":"EUR","itemDesc":"Coffee"}
GET  /api/v1/invoices/{id}
POST /api/v1/invoices/{id}/claim  {"txid":"..."}
```

Webhook: POST invoice JSON to the URL you set. Header `X-Ishum-Event`.

`ISHUM_DEMO=1` allows marking the live KAS rail settled without a tx. Reserved rails can be demo-settled without that flag.

Listen: `ISHUM_ADDR` (default `127.0.0.1:8090`). Data: `ISHUM_DATA` (default `./data`).

PoC revisited: [POC-REVISITED.md](POC-REVISITED.md) · [STP-KAS/poc-revisited](https://github.com/STP-KAS/poc-revisited) · till on [sixpack.wtf/till.html](https://sixpack.wtf/till.html)

Explainer (BTCPay 1:33 structure, facts VO): `video/Ishum-sequenced-on-Kaspa.mp4`
