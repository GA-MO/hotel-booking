package notification

import (
	"bytes"
	"fmt"
	htmltmpl "html/template"
	"regexp"
	"strings"
	texttmpl "text/template"
)

// Templates are kept inline as constants — no embed.FS, no external files.
// Each (template, locale) pair has three pieces:
//   - subject:  text/template — no HTML
//   - bodyHTML: html/template — escaped output for email
//   - bodyText: derived from bodyHTML by stripping tags (good enough Phase 1)
//
// All templates are intentionally minimal — guests open mail on phones.

type tpl struct {
	subjectTH string
	subjectEN string
	bodyTH    string
	bodyEN    string
}

var templates = map[Template]tpl{
	TemplateBookingCreated: {
		subjectTH: `เรากำลังรอการชำระเงินสำหรับการจอง {{.reference}}`,
		subjectEN: `We're holding your booking {{.reference}}`,
		bodyTH: `<p>สวัสดีคุณ {{.guest_name}},</p>
<p>เราได้รับการจองของคุณที่ <strong>{{.hotel_name}}</strong> เรียบร้อยแล้ว</p>
<ul>
  <li>หมายเลขการจอง: <strong>{{.reference}}</strong></li>
  <li>วันเข้าพัก: {{.check_in_date}} ถึง {{.check_out_date}} ({{.nights}} คืน)</li>
  <li>ยอดรวม: {{.total}} {{.currency}}</li>
</ul>
<p>กรุณาชำระเงินภายใน 10 นาทีเพื่อยืนยันการจอง</p>`,
		bodyEN: `<p>Hello {{.guest_name}},</p>
<p>We have received your booking at <strong>{{.hotel_name}}</strong>.</p>
<ul>
  <li>Reference: <strong>{{.reference}}</strong></li>
  <li>Stay: {{.check_in_date}} to {{.check_out_date}} ({{.nights}} nights)</li>
  <li>Total: {{.total}} {{.currency}}</li>
</ul>
<p>Please complete payment within 10 minutes to confirm the booking.</p>`,
	},

	TemplateBookingConfirmed: {
		subjectTH: `ยืนยันการจอง {{.reference}}`,
		subjectEN: `Booking confirmed: {{.reference}}`,
		bodyTH: `<p>สวัสดีคุณ {{.guest_name}},</p>
<p>การจองของคุณที่ <strong>{{.hotel_name}}</strong> ได้รับการยืนยันแล้ว</p>
<ul>
  <li>หมายเลขการจอง: <strong>{{.reference}}</strong></li>
  <li>วันเข้าพัก: {{.check_in_date}} ถึง {{.check_out_date}} ({{.nights}} คืน)</li>
  <li>ยอดที่ชำระ: {{.total}} {{.currency}}</li>
</ul>
<p>เราตั้งตารอที่จะต้อนรับคุณ</p>`,
		bodyEN: `<p>Hello {{.guest_name}},</p>
<p>Your booking at <strong>{{.hotel_name}}</strong> is confirmed.</p>
<ul>
  <li>Reference: <strong>{{.reference}}</strong></li>
  <li>Stay: {{.check_in_date}} to {{.check_out_date}} ({{.nights}} nights)</li>
  <li>Paid: {{.total}} {{.currency}}</li>
</ul>
<p>We look forward to welcoming you.</p>`,
	},

	TemplateBookingCancelled: {
		subjectTH: `ยกเลิกการจอง {{.reference}}`,
		subjectEN: `Booking cancelled: {{.reference}}`,
		bodyTH: `<p>สวัสดีคุณ {{.guest_name}},</p>
<p>การจองของคุณ <strong>{{.reference}}</strong> ที่ {{.hotel_name}} ถูกยกเลิกแล้ว</p>
{{if .reason}}<p>เหตุผล: {{.reason}}</p>{{end}}
<p>หากมีข้อสงสัย กรุณาติดต่อโรงแรมโดยตรง</p>`,
		bodyEN: `<p>Hello {{.guest_name}},</p>
<p>Your booking <strong>{{.reference}}</strong> at {{.hotel_name}} has been cancelled.</p>
{{if .reason}}<p>Reason: {{.reason}}</p>{{end}}
<p>If you have any questions, please contact the hotel directly.</p>`,
	},

	TemplateBookingCheckInReminder: {
		subjectTH: `พรุ่งนี้คุณเข้าพักที่ {{.hotel_name}}`,
		subjectEN: `Reminder: your stay at {{.hotel_name}} starts tomorrow`,
		bodyTH: `<p>สวัสดีคุณ {{.guest_name}},</p>
<p>นี่คือเครื่องเตือนความจำสำหรับการจอง <strong>{{.reference}}</strong></p>
<ul>
  <li>เช็คอิน: {{.check_in_date}} เวลา {{.check_in_time}}</li>
  <li>เช็คเอาท์: {{.check_out_date}}</li>
  <li>ที่อยู่: {{.hotel_address}}</li>
</ul>`,
		bodyEN: `<p>Hello {{.guest_name}},</p>
<p>A friendly reminder about your booking <strong>{{.reference}}</strong>.</p>
<ul>
  <li>Check-in: {{.check_in_date}} at {{.check_in_time}}</li>
  <li>Check-out: {{.check_out_date}}</li>
  <li>Address: {{.hotel_address}}</li>
</ul>`,
	},

	TemplateBookingPostStay: {
		subjectTH: `ขอบคุณที่เข้าพักกับ {{.hotel_name}}`,
		subjectEN: `Thanks for staying at {{.hotel_name}}`,
		bodyTH: `<p>สวัสดีคุณ {{.guest_name}},</p>
<p>ขอบคุณที่เลือกพักกับเรา หากสะดวก ขอความกรุณารีวิวประสบการณ์ของคุณ</p>
{{if .review_url}}<p><a href="{{.review_url}}">เขียนรีวิว</a></p>{{end}}`,
		bodyEN: `<p>Hello {{.guest_name}},</p>
<p>Thank you for staying with us. We'd love it if you could share your experience.</p>
{{if .review_url}}<p><a href="{{.review_url}}">Leave a review</a></p>{{end}}`,
	},
}

// RenderEmail returns subject, html, and text bodies for the named template,
// honouring payload["locale"] (defaults to "th"). The text body is derived
// from the html by stripping tags — fine for Phase 1, multipart emails will
// add a dedicated text template later.
func RenderEmail(template Template, payload map[string]any) (subject, html, text string, err error) {
	t, ok := templates[template]
	if !ok {
		return "", "", "", fmt.Errorf("%w: %s", ErrInvalidTemplate, template)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	locale := strings.ToLower(stringValue(payload, "locale"))
	if locale == "" {
		locale = "th"
	}

	subjSrc := t.subjectTH
	bodySrc := t.bodyTH
	if locale == "en" {
		subjSrc = t.subjectEN
		bodySrc = t.bodyEN
	}

	subjBuf := &bytes.Buffer{}
	subjTpl, err := texttmpl.New("subject").Option("missingkey=zero").Parse(subjSrc)
	if err != nil {
		return "", "", "", fmt.Errorf("%w: parse subject: %v", ErrTemplateRender, err)
	}
	if err := subjTpl.Execute(subjBuf, payload); err != nil {
		return "", "", "", fmt.Errorf("%w: execute subject: %v", ErrTemplateRender, err)
	}

	bodyBuf := &bytes.Buffer{}
	bodyTpl, err := htmltmpl.New("body").Option("missingkey=zero").Parse(bodySrc)
	if err != nil {
		return "", "", "", fmt.Errorf("%w: parse body: %v", ErrTemplateRender, err)
	}
	if err := bodyTpl.Execute(bodyBuf, payload); err != nil {
		return "", "", "", fmt.Errorf("%w: execute body: %v", ErrTemplateRender, err)
	}

	subject = strings.TrimSpace(subjBuf.String())
	html = strings.TrimSpace(bodyBuf.String())
	text = stripHTML(html)
	return subject, html, text, nil
}

var tagRe = regexp.MustCompile(`<[^>]+>`)

// stripHTML removes tags and collapses whitespace. Adequate for the Phase 1
// minimal templates — none use <table>, <style> etc.
func stripHTML(html string) string {
	noTags := tagRe.ReplaceAllString(html, "")
	// Collapse repeated whitespace to single spaces, preserving newlines
	// between paragraphs by keeping double newlines.
	lines := strings.Split(noTags, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		trim := strings.TrimSpace(l)
		if trim != "" {
			out = append(out, trim)
		}
	}
	return strings.Join(out, "\n")
}

func stringValue(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
