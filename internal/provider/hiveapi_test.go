package provider

import "testing"

// TestHiveAPIProviderRegistered pins the Hivex Gateway as a first-class
// runtime: registered, pickable (not gateway-only), and compat-dispatched.
func TestHiveAPIProviderRegistered(t *testing.T) {
	if err := ValidateKind(KindHiveAPI); err != nil {
		t.Fatalf("ValidateKind(hiveapi): %v", err)
	}
	if Lookup(KindHiveAPI) == nil {
		t.Fatal("hiveapi must be registered")
	}
	found := false
	for _, k := range LLMProviderKinds() {
		if k == KindHiveAPI {
			found = true
		}
	}
	if !found {
		t.Fatal("hiveapi must appear in the runtime picker list (LLMProviderKinds)")
	}
	for _, k := range GatewayKinds() {
		if k == KindHiveAPI {
			t.Fatal("hiveapi is a directly-runnable runtime, not a gateway-only kind")
		}
	}
}
