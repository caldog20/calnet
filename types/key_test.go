package types

import (
	"encoding/json"
	"testing"
)

func TestMarshalJSONPublicKey(t *testing.T) {
	pr := NewPrivateKey()
	pub1 := pr.Public()
	b, err := json.Marshal(pub1)
	if err != nil {
		t.Fatal(err)
	}
	pub2 := PublicKey{}
	err = json.Unmarshal(b, &pub2)
	if err != nil {
		t.Fatal(err)
	}
	if pub1 != pub2 {
		t.Fatal("pub1 and pub2 should be the same")
	}
}
