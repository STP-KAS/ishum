# BTCPay plugin path (not this PoC)

BTCPay’s documented altcoin path is a C# plugin plus a docker-compose fragment.

Required (from https://docs.btcpayserver.org/Development/Altcoins/):

1. Plugin extending `BaseBTCPayServerPlugin`
2. `IPaymentMethodHandler` (see Monero: `MoneroLikePaymentMethodHandler`)
3. A listener that watches the coin’s daemon/wallet RPC (`MoneroListener`)
4. Docker fragment for `kaspad` + wallet RPC
5. Listing on the Plugin Builder only after the docker PR merges

Kaspa does not speak Bitcoin script. NBXplorer will not see KAS. The plugin would be a Monero-shaped custom listener, not a config flag.

This folder is the mapping, not a loadable DLL.

| BTCPay | Ishum today |
| --- | --- |
| Invoice engine | `internal/invoice` |
| NBXplorer | `internal/watch` → `api.kaspa.org` |
| Payment method | `internal/rails` (`kas`, `kusd`, `usdt`) |
| Greenfield `POST /api/v1/stores/{id}/invoices` | `POST /api/v1/invoices` |
| POS app | `/pos` |
| Pay button | `/button` |
| Webhook `InvoiceSettled` | `X-Ishum-Event` |

A later plugin should treat Ishum as the Kaspa watcher (mini-NBXplorer) and register three **settlement** methods on the BTCPay invoice. Do not fork BTCPay core. Do not register USDT as gas. Sequence and fees stay KAS.

Why the plugin is not the PoC: it needs BTCPay + Postgres + Bitcoin Core + a Kaspa node + .NET 10, and the two stable rails have no issued asset to pay. The till would not run today.
