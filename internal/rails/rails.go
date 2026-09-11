// Package rails is settlement, not consensus.
//
// Sequence and miner fees stay KAS. The till quotes a stable unit (EUR/USD).
// Settlement is a separate object:
//
//	kas  — native. No issuer. Volatile. Live.
//	kusd — reserved native-stable. Needs capital. Not Tether. Not live.
//	usdt — guest IOU. Tether prints, burns, freezes. Does not vote in GHOSTDAG.
package rails

const (
	KAS  = "kas"
	KUSD = "kusd"
	USDT = "usdt"

	KindNative   = "native"
	KindReserved = "reserved-native"
	KindGuest    = "guest-iou"

	Sequence = "kaspa-l1"
	FeeAsset = "KAS"
)

type Info struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Unit     string `json:"unit"`
	Decimals int    `json:"decimals"`
	Live     bool   `json:"live"`
	Kind     string `json:"kind"`
	Guest    bool   `json:"guest"`
	Freeze   bool   `json:"freeze"`
	Issuer   string `json:"issuer,omitempty"`
	Badge    string `json:"badge"`
	Note     string `json:"note"`
}

var All = []Info{
	{
		ID: KAS, Name: "Kaspa native", Unit: "KAS", Decimals: 8,
		Live: true, Kind: KindNative, Badge: "native · live",
		Note: "No issuer. Volatile. This is why the till quotes EUR/USD. L1 KAS to the merchant address.",
	},
	{
		ID: KUSD, Name: "Kaspa stable", Unit: "kUSD", Decimals: 6,
		Live: false, Kind: KindReserved, Badge: "reserved · needs capital",
		Note: "Native-stable slot if one is issued on Kaspa. Overcollateral or reserves. Not Tether. No free dollar. Not live.",
	},
	{
		ID: USDT, Name: "USDT (guest)", Unit: "USDT", Decimals: 6,
		Live: false, Kind: KindGuest, Guest: true, Freeze: true,
		Issuer: "Tether", Badge: "guest IOU · freeze",
		Note: "Custodial credit on a Kaspa rail. Tether prints, burns, freezes, answers to courts. Does not vote in GHOSTDAG. Not gas. Not the constitution.",
	},
}

func Get(id string) (Info, bool) {
	for _, r := range All {
		if r.ID == id {
			return r, true
		}
	}
	return Info{}, false
}
