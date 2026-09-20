package team

// cognee_backend_live_test.go — env-gated LIVE round-trip against a real
// Cognee instance (HIVEX_TEST_COGNEE_URL, e.g. http://localhost:8090).
// Skipped unless the env is set, so CI/default runs stay offline.

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCogneeLiveClientRoundTrip(t *testing.T) {
	liveURL := strings.TrimSpace(os.Getenv("HIVEX_TEST_COGNEE_URL"))
	if liveURL == "" {
		t.Skip("set HIVEX_TEST_COGNEE_URL to run the live cognee round-trip")
	}
	t.Setenv("HIVEX_COGNEE_URL", liveURL)
	c := newCogneeHTTPClient()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if !c.Ready(ctx) {
		t.Fatalf("live cognee at %s is not ready", liveURL)
	}

	marker := "Live probe " + time.Now().Format("150405.000") + ": the Hivex Harness writes team memory through the cognee backend."
	id, err := c.Remember(ctx, marker, "probe")
	if err != nil {
		t.Fatalf("live remember: %v", err)
	}
	if strings.TrimSpace(id) == "" {
		t.Fatal("live remember returned an empty run reference")
	}

	// Retrieval is eventual: the pipeline runs synchronously per the add
	// call, but allow a short poll for the index to settle.
	var hits []cogneeHit
	for attempt := 0; attempt < 10; attempt++ {
		hits, err = c.Search(ctx, "What does the Hivex Harness write through?", 5)
		if err != nil {
			t.Fatalf("live search: %v", err)
		}
		for _, h := range hits {
			if strings.Contains(h.Text, "Live probe") {
				return // round-trip proven
			}
		}
		time.Sleep(3 * time.Second)
	}
	t.Fatalf("marker not retrieved after write; hits=%d", len(hits))
}
