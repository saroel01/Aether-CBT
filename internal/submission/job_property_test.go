package submission

// Feature: filesystem-submission-queue, Property 2

import (
	"bytes"
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"pgregory.net/rapid"
)

// genSubmissionJob generates a random SubmissionJob.
// DetailXML is 50% empty, 50% XML string containing special characters.
//
// Validates: Requirements 1.5, 10.5, 11.4
func genSubmissionJob(t *rapid.T) *SubmissionJob {
	// TenantID: random positive int (1..1000)
	tenantID := rapid.IntRange(1, 1000).Draw(t, "tenant_id")

	// NoID: random non-empty string
	noID := rapid.StringMatching(`[A-Za-z0-9_-]{1,20}`).Draw(t, "no_id")

	// Validasi: random non-empty string
	validasi := rapid.StringMatching(`[A-Za-z0-9_-]{1,30}`).Draw(t, "validasi")

	// Score, MaxScore: random strings (may be empty)
	score := rapid.String().Draw(t, "score")
	maxScore := rapid.String().Draw(t, "max_score")

	// AttemptToken: random string
	attemptToken := rapid.String().Draw(t, "attempt_token")

	// EnqueuedAt: random time, normalized to UTC, truncated to seconds
	// Use a range of unix timestamps to avoid zero time (which fails validation)
	unixSec := rapid.Int64Range(1_000_000_000, 2_000_000_000).Draw(t, "enqueued_at_unix")
	enqueuedAt := time.Unix(unixSec, 0).UTC()

	// RetryCount: random int 0..10
	retryCount := rapid.IntRange(0, 10).Draw(t, "retry_count")

	// LastError: random string
	lastError := rapid.String().Draw(t, "last_error")

	// DetailXML: 50% empty, 50% XML with special characters
	var detailXML string
	useXML := rapid.Bool().Draw(t, "use_xml")
	if useXML {
		// Build XML string with special characters: <, >, &, \n, ", unicode
		specialChars := []string{
			`<results><question id="q1" status="correct" score="1" /></results>`,
			"<data>\n  <item value=\"hello &amp; world\" />\n</data>",
			"<root>\n  <text>Special: &lt;tag&gt; &amp; \"quoted\" \u00e9\u00e0\u00fc</text>\n</root>",
			"<r><a b=\"&lt;&gt;&amp;&quot;\">value\n</a></r>",
			"<unicode>\u4e2d\u6587\u6d4b\u8bd5 \u00e9\u00e0\u00fc</unicode>",
		}
		idx := rapid.IntRange(0, len(specialChars)-1).Draw(t, "xml_variant")
		detailXML = specialChars[idx]
	}

	return &SubmissionJob{
		TenantID:     tenantID,
		NoID:         noID,
		Validasi:     validasi,
		Score:        score,
		MaxScore:     maxScore,
		AttemptToken: attemptToken,
		EnqueuedAt:   enqueuedAt,
		RetryCount:   retryCount,
		LastError:    lastError,
		DetailXML:    detailXML,
	}
}

// TestPropertyRoundTripSerialization verifies Property 2: Round-trip Serialization.
//
// For any valid SubmissionJob j, UnmarshalJob(MarshalJob(j)) must be field-by-field
// equivalent to j (with EnqueuedAt normalized to UTC).
//
// Validates: Requirements 1.5, 10.5, 11.4
func TestPropertyRoundTripSerialization(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		original := genSubmissionJob(t)

		// Marshal the job to JSON
		data, err := MarshalJob(original)
		if err != nil {
			t.Fatalf("MarshalJob failed: %v", err)
		}

		// Unmarshal back
		restored, err := UnmarshalJob(data)
		if err != nil {
			t.Fatalf("UnmarshalJob failed: %v", err)
		}

		// Normalize EnqueuedAt to UTC for comparison (both should already be UTC,
		// but ensure monotonic clock is stripped for DeepEqual)
		originalNormalized := *original
		originalNormalized.EnqueuedAt = original.EnqueuedAt.UTC().Truncate(time.Second)

		restoredNormalized := *restored
		restoredNormalized.EnqueuedAt = restored.EnqueuedAt.UTC().Truncate(time.Second)

		// Compare field-by-field using reflect.DeepEqual
		if !reflect.DeepEqual(originalNormalized, restoredNormalized) {
			t.Fatalf("round-trip mismatch:\n  original:  %+v\n  restored:  %+v",
				originalNormalized, restoredNormalized)
		}
	})
}

// TestPropertyMarshalJobFormat verifies Property 3: Format Output MarshalJob.
//
// For any valid SubmissionJob, MarshalJob must produce output that:
// (a) is parseable by standard json.Unmarshal without error,
// (b) has each non-first line starting with at least 2 spaces (indent),
// (c) has a fixed key order: validasi, tenant_id, no_id, score, max_score,
//
//	attempt_token, enqueued_at, retry_count, last_error, detail_xml,
//
// (d) has enqueued_at matching ISO 8601 UTC regex.
//
// Validates: Requirements 10.1, 10.2, 11.5
func TestPropertyMarshalJobFormat(t *testing.T) {
	// Expected key order per Requirement 10.1
	expectedKeyOrder := []string{
		"validasi",
		"tenant_id",
		"no_id",
		"score",
		"max_score",
		"attempt_token",
		"enqueued_at",
		"retry_count",
		"last_error",
		"detail_xml",
	}

	// ISO 8601 UTC regex per Requirement 10.2
	iso8601UTC := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z$`)

	rapid.Check(t, func(t *rapid.T) {
		job := genSubmissionJob(t)

		data, err := MarshalJob(job)
		if err != nil {
			t.Fatalf("MarshalJob failed: %v", err)
		}

		// (a) Standard json.Unmarshal succeeds without error
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("json.Unmarshal failed: %v", err)
		}

		// (b) Each non-first line starts with at least 2 spaces
		lines := strings.Split(string(data), "\n")
		for i, line := range lines[1:] {
			// Skip empty lines (e.g. trailing newline after closing brace)
			if line == "" {
				continue
			}
			// The closing brace "}" is the last non-empty line and has no indent
			// (it's the root-level closing brace). All other non-first lines
			// that are not the root closing brace must be indented.
			if line == "}" {
				continue
			}
			if !strings.HasPrefix(line, "  ") {
				t.Fatalf("line %d does not start with 2 spaces: %q", i+1, line)
			}
		}

		// (c) Key order is fixed using json.Decoder Token() API
		dec := json.NewDecoder(bytes.NewReader(data))

		// Consume opening '{'
		tok, err := dec.Token()
		if err != nil {
			t.Fatalf("decoder Token() failed reading '{': %v", err)
		}
		if delim, ok := tok.(json.Delim); !ok || delim != '{' {
			t.Fatalf("expected '{', got %v", tok)
		}

		var actualKeys []string
		for dec.More() {
			// Read key token
			keyTok, err := dec.Token()
			if err != nil {
				t.Fatalf("decoder Token() failed reading key: %v", err)
			}
			key, ok := keyTok.(string)
			if !ok {
				t.Fatalf("expected string key, got %T: %v", keyTok, keyTok)
			}
			actualKeys = append(actualKeys, key)

			// Skip the value (may be nested, use Decoder to handle it)
			var val interface{}
			if err := dec.Decode(&val); err != nil {
				t.Fatalf("decoder Decode() failed reading value for key %q: %v", key, err)
			}
		}

		if !reflect.DeepEqual(actualKeys, expectedKeyOrder) {
			t.Fatalf("key order mismatch:\n  expected: %v\n  actual:   %v", expectedKeyOrder, actualKeys)
		}

		// (d) enqueued_at value matches ISO 8601 UTC regex
		enqueuedAtVal, ok := raw["enqueued_at"]
		if !ok {
			t.Fatalf("enqueued_at key missing from marshaled JSON")
		}
		enqueuedAtStr, ok := enqueuedAtVal.(string)
		if !ok {
			t.Fatalf("enqueued_at is not a string: %T", enqueuedAtVal)
		}
		if !iso8601UTC.MatchString(enqueuedAtStr) {
			t.Fatalf("enqueued_at %q does not match ISO 8601 UTC regex", enqueuedAtStr)
		}
	})
}
