package rails

import "testing"

func TestKinds(t *testing.T) {
	kas, _ := Get(KAS)
	if kas.Guest || kas.Freeze || kas.Kind != KindNative || !kas.Live {
		t.Fatalf("kas %+v", kas)
	}
	usd, _ := Get(USDT)
	if !usd.Guest || !usd.Freeze || usd.Issuer != "Tether" || usd.Live {
		t.Fatalf("usdt %+v", usd)
	}
	k, _ := Get(KUSD)
	if k.Guest || k.Live || k.Kind != KindReserved {
		t.Fatalf("kusd %+v", k)
	}
	if Sequence != "kaspa-l1" || FeeAsset != "KAS" {
		t.Fatal("constitution")
	}
}
