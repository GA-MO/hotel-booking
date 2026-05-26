package notification

import (
	"strings"
	"testing"
)

func TestRenderEmail_AllTemplatesBothLocales(t *testing.T) {
	t.Parallel()
	templates := []Template{
		TemplateBookingCreated,
		TemplateBookingConfirmed,
		TemplateBookingCancelled,
		TemplateBookingCheckInReminder,
		TemplateBookingPostStay,
	}
	locales := []string{"th", "en"}

	for _, tpl := range templates {
		for _, loc := range locales {
			tpl, loc := tpl, loc
			t.Run(string(tpl)+"/"+loc, func(t *testing.T) {
				t.Parallel()
				payload := samplePayload(loc)
				subject, html, text, err := RenderEmail(tpl, payload)
				if err != nil {
					t.Fatalf("render: %v", err)
				}
				if strings.TrimSpace(subject) == "" {
					t.Fatalf("empty subject for %s/%s", tpl, loc)
				}
				if strings.TrimSpace(html) == "" {
					t.Fatalf("empty html for %s/%s", tpl, loc)
				}
				if strings.TrimSpace(text) == "" {
					t.Fatalf("empty text for %s/%s", tpl, loc)
				}
				// Substitution sanity: reference must appear in the body of
				// every booking template (or in the subject — booking_post_stay
				// doesn't include it intentionally).
				if tpl != TemplateBookingPostStay {
					combined := subject + " " + html
					if !strings.Contains(combined, "REF-123") {
						t.Fatalf("expected reference in output for %s/%s, got subject=%q html=%q", tpl, loc, subject, html)
					}
				}
				// Hotel name should appear in subject or body for every template.
				combined := subject + " " + html
				if !strings.Contains(combined, "Test Hotel") {
					t.Fatalf("expected hotel_name in output for %s/%s, got subject=%q", tpl, loc, subject)
				}
				// Text body should have no HTML tags.
				if strings.Contains(text, "<") || strings.Contains(text, ">") {
					t.Fatalf("text body still has tags: %q", text)
				}
			})
		}
	}
}

func TestRenderEmail_UnknownTemplate(t *testing.T) {
	t.Parallel()
	_, _, _, err := RenderEmail(Template("no_such_template"), samplePayload("th"))
	if err == nil {
		t.Fatalf("expected error for unknown template")
	}
}

func TestRenderEmail_DefaultsToThai(t *testing.T) {
	t.Parallel()
	payload := samplePayload("")
	delete(payload, "locale")
	subjectTH, _, _, err := RenderEmail(TemplateBookingConfirmed, payload)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	// Pull the EN one for comparison.
	payload["locale"] = "en"
	subjectEN, _, _, err := RenderEmail(TemplateBookingConfirmed, payload)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if subjectTH == subjectEN {
		t.Fatalf("expected default locale to be 'th', got identical subjects: %q", subjectTH)
	}
}

func TestRenderEmail_CancellationWithReason(t *testing.T) {
	t.Parallel()
	payload := samplePayload("en")
	payload["reason"] = "Guest requested cancellation"
	_, html, _, err := RenderEmail(TemplateBookingCancelled, payload)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !strings.Contains(html, "Guest requested cancellation") {
		t.Fatalf("expected reason in body, got: %s", html)
	}
}

func samplePayload(locale string) map[string]any {
	return map[string]any{
		"locale":         locale,
		"reference":      "REF-123",
		"hotel_name":     "Test Hotel",
		"hotel_address":  "123 Test Street",
		"guest_name":     "Sample Guest",
		"check_in_date":  "2026-06-01",
		"check_in_time":  "15:00",
		"check_out_date": "2026-06-03",
		"nights":         2,
		"currency":       "THB",
		"total":          "2400.00",
		"review_url":     "https://example.com/review/REF-123",
	}
}
