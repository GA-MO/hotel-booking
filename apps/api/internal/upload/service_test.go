package upload

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	signer := NewSigner(SignerConfig{
		Endpoint:  "http://minio:9000",
		Region:    "us-east-1",
		Bucket:    "hotel-photos",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
	})
	if signer == nil {
		t.Fatal("signer nil")
	}
	imgp, err := NewImgproxy(ImgproxyConfig{
		BaseURL: "http://imgproxy:8080",
		// hex-encoded "secretkey" / "saltsalt"
		Key:  "73656372657466746b6579", // not a real value; test only
		Salt: "73616c7473616c74",
	})
	if err != nil {
		t.Fatalf("NewImgproxy: %v", err)
	}
	return NewService(ServiceConfig{
		Signer:        signer,
		Imgproxy:      imgp,
		Bucket:        "hotel-photos",
		PublicBaseURL: "http://localhost:9000/hotel-photos",
		Expires:       15 * time.Minute,
	})
}

func TestPresign_ObjectKeyShape(t *testing.T) {
	t.Parallel()
	svc := newTestService(t)

	acct := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	hot := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	resp, err := svc.Presign(context.Background(), acct, PresignRequest{
		Kind:        KindHotelPhoto,
		HotelID:     &hot,
		ContentType: "image/jpeg",
		SizeBytes:   1024,
	})
	if err != nil {
		t.Fatalf("Presign: %v", err)
	}

	// {account_id}/{hotel_id}/{kind}/{uuid}.jpg
	prefix := acct.String() + "/" + hot.String() + "/hotel_photo/"
	if !strings.HasPrefix(resp.ObjectKey, prefix) {
		t.Errorf("ObjectKey missing prefix:\n got: %s\nwant prefix: %s", resp.ObjectKey, prefix)
	}
	if !strings.HasSuffix(resp.ObjectKey, ".jpg") {
		t.Errorf("ObjectKey missing .jpg suffix: %s", resp.ObjectKey)
	}

	wantPub := "http://localhost:9000/hotel-photos/" + resp.ObjectKey
	if resp.PublicURL != wantPub {
		t.Errorf("PublicURL: got %s, want %s", resp.PublicURL, wantPub)
	}
	if !strings.HasPrefix(resp.UploadURL, "http://minio:9000/hotel-photos/") {
		t.Errorf("UploadURL prefix unexpected: %s", resp.UploadURL)
	}
	if resp.Headers["Content-Type"] != "image/jpeg" {
		t.Errorf("expected Content-Type=image/jpeg, got %v", resp.Headers)
	}
}

func TestPresign_NoHotelIDFallsBack(t *testing.T) {
	t.Parallel()
	svc := newTestService(t)
	acct := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	resp, err := svc.Presign(context.Background(), acct, PresignRequest{
		Kind:        KindHotelPhoto,
		ContentType: "image/png",
		SizeBytes:   1024,
	})
	if err != nil {
		t.Fatalf("Presign: %v", err)
	}
	if !strings.Contains(resp.ObjectKey, "/no-hotel/") {
		t.Errorf("expected /no-hotel/ segment, got %s", resp.ObjectKey)
	}
	if !strings.HasSuffix(resp.ObjectKey, ".png") {
		t.Errorf("expected .png ext, got %s", resp.ObjectKey)
	}
}

func TestPresign_Validation(t *testing.T) {
	t.Parallel()
	svc := newTestService(t)
	acct := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	cases := []struct {
		name string
		req  PresignRequest
		want error
	}{
		{
			name: "unknown kind",
			req:  PresignRequest{Kind: "video", ContentType: "image/jpeg", SizeBytes: 100},
			want: ErrUnsupportedKind,
		},
		{
			name: "disallowed content type",
			req:  PresignRequest{Kind: KindHotelPhoto, ContentType: "image/gif", SizeBytes: 100},
			want: ErrInvalidContentType,
		},
		{
			name: "size zero",
			req:  PresignRequest{Kind: KindHotelPhoto, ContentType: "image/jpeg", SizeBytes: 0},
			want: ErrSizeExceeded,
		},
		{
			name: "size negative",
			req:  PresignRequest{Kind: KindHotelPhoto, ContentType: "image/jpeg", SizeBytes: -1},
			want: ErrSizeExceeded,
		},
		{
			name: "size over cap",
			req:  PresignRequest{Kind: KindHotelPhoto, ContentType: "image/jpeg", SizeBytes: MaxUploadBytes + 1},
			want: ErrSizeExceeded,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, err := svc.Presign(context.Background(), acct, c.req)
			if !errors.Is(err, c.want) {
				t.Fatalf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestPresign_NotConfigured(t *testing.T) {
	t.Parallel()
	svc := NewService(ServiceConfig{}) // no signer
	_, err := svc.Presign(context.Background(), uuid.New(), PresignRequest{
		Kind: KindHotelPhoto, ContentType: "image/jpeg", SizeBytes: 100,
	})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("got %v, want ErrNotConfigured", err)
	}
}

func TestExtensionFor(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"image/jpeg": "jpg",
		"image/png":  "png",
		"image/webp": "webp",
	}
	for ct, want := range cases {
		if got := extensionFor(ct); got != want {
			t.Errorf("extensionFor(%q) = %q, want %q", ct, got, want)
		}
	}
}

// ----- imgproxy tests -----

func TestImgproxy_InsecureModeWhenKeyMissing(t *testing.T) {
	t.Parallel()
	i, err := NewImgproxy(ImgproxyConfig{BaseURL: "http://imgproxy:8080"})
	if err != nil {
		t.Fatalf("NewImgproxy: %v", err)
	}
	got := i.Sign("http://minio:9000/hotel-photos/a/b/c.jpg", Transform{Width: 800})
	if !strings.HasPrefix(got, "http://imgproxy:8080/insecure/") {
		t.Errorf("expected /insecure/ prefix in dev mode, got %s", got)
	}
}

func TestImgproxy_SignsWithHMAC(t *testing.T) {
	t.Parallel()
	// Use known short hex values to keep the signature manageable.
	i, err := NewImgproxy(ImgproxyConfig{
		BaseURL: "http://imgproxy:8080",
		Key:     "00",
		Salt:    "00",
	})
	if err != nil {
		t.Fatalf("NewImgproxy: %v", err)
	}
	got := i.Sign("http://minio:9000/hotel-photos/a/b/c.jpg", Transform{Width: 800, Height: 600})
	// Expect: http://imgproxy:8080/{sig}/rs:fit:800:600/{base64url(source)}
	if !strings.HasPrefix(got, "http://imgproxy:8080/") {
		t.Fatalf("unexpected URL: %s", got)
	}
	if !strings.Contains(got, "/rs:fit:800:600/") {
		t.Errorf("missing /rs:fit:800:600/ segment: %s", got)
	}
	if strings.Contains(got, "/insecure/") {
		t.Errorf("expected signed URL, got insecure: %s", got)
	}
}

func TestImgproxy_FormatAppendedAsExtension(t *testing.T) {
	t.Parallel()
	i, _ := NewImgproxy(ImgproxyConfig{
		BaseURL: "http://imgproxy:8080",
		Key:     "deadbeef",
		Salt:    "cafebabe",
	})
	got := i.Sign("http://x/a.jpg", Transform{Width: 400, Format: "webp"})
	if !strings.HasSuffix(got, ".webp") {
		t.Errorf("expected .webp ext on encoded source, got %s", got)
	}
}

func TestImgproxy_PlainSegmentWhenNoTransform(t *testing.T) {
	t.Parallel()
	i, _ := NewImgproxy(ImgproxyConfig{BaseURL: "http://imgproxy:8080"})
	got := i.Sign("http://x/a.jpg", Transform{})
	if !strings.Contains(got, "/plain/") {
		t.Errorf("expected /plain/ segment when no transform set, got %s", got)
	}
}

func TestImgproxy_InvalidHexKeyError(t *testing.T) {
	t.Parallel()
	_, err := NewImgproxy(ImgproxyConfig{Key: "not-hex!", Salt: "00"})
	if err == nil {
		t.Fatal("expected error for invalid hex in Key")
	}
}

// ----- error mapping -----

func TestMapErrorCode_Sentinels(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"unsupported kind", ErrUnsupportedKind, 400, "UNSUPPORTED_KIND"},
		{"invalid content type", ErrInvalidContentType, 400, "INVALID_CONTENT_TYPE"},
		{"size exceeded", ErrSizeExceeded, 400, "SIZE_EXCEEDED"},
		{"not configured", ErrNotConfigured, 503, "STORAGE_UNAVAILABLE"},
		{"unknown", errors.New("boom"), 500, "INTERNAL"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			status, code, _ := mapErrorCode(c.err)
			if status != c.wantStatus || code != c.wantCode {
				t.Errorf("got (%d, %q), want (%d, %q)", status, code, c.wantStatus, c.wantCode)
			}
		})
	}
}
