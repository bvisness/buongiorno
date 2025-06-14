package src

import "testing"

func TestGetAvahiServiceTypes(t *testing.T) {
	types := getAvahiServiceTypes()
	for _, typ := range types {
		t.Logf("%s (%s)", typ.DNSSDName, typ.NiceName)
	}
}
