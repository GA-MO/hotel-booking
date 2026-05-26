package booking_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/GA-MO/hotel-booking/apps/api/internal/booking"
	"github.com/GA-MO/hotel-booking/apps/api/internal/testdb"
)

// TestRepo_CreatePending_HappyPath verifies the basic insert + booking_event
// audit trail under a real database.
func TestRepo_CreatePending_HappyPath(t *testing.T) {
	pool := testdb.Open(t)
	testdb.Truncate(t, pool)
	f := testdb.SeedBase(t, pool, 3)
	repo := booking.NewRepository(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	checkIn := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	checkOut := checkIn.AddDate(0, 0, 2)

	b := &booking.Booking{
		HotelID:           f.HotelID,
		RoomTypeID:        f.RoomTypeID,
		RoomCount:         1,
		GuestEmail:        "guest@example.com",
		GuestName:         "Smoke Guest",
		CheckInDate:       checkIn,
		CheckOutDate:      checkOut,
		Currency:          "THB",
		RoomSubtotalCents: 300000,
		TotalCents:        300000,
		Source:            booking.SourceWeb,
	}

	got, err := repo.CreatePending(ctx, b, 10*time.Minute, f.RoomTypeID)
	if err != nil {
		t.Fatalf("CreatePending: %v", err)
	}
	if got.Status != booking.StatusPendingPayment {
		t.Errorf("status: want %q, got %q", booking.StatusPendingPayment, got.Status)
	}
	if got.Reference == "" || got.Reference[:3] != "HB-" {
		t.Errorf("reference: want HB-…, got %q", got.Reference)
	}
	if got.ExpiresAt == nil || got.ExpiresAt.Before(time.Now().Add(8*time.Minute)) {
		t.Errorf("expires_at not set forward: %v", got.ExpiresAt)
	}

	// Verify booking_events row.
	var eventCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM booking_events WHERE booking_id = $1 AND event_type = 'created'
	`, got.ID).Scan(&eventCount); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if eventCount != 1 {
		t.Errorf("created events: want 1, got %d", eventCount)
	}
}

// TestRepo_CreatePending_RaceNoOversell is THE critical correctness test:
// kicks off N concurrent goroutines all trying to book the same dates against
// an inventory of capacity rooms. Only `capacity` bookings must succeed.
//
// Without the FOR UPDATE row lock + in-transaction availability count this
// test reliably oversells.
func TestRepo_CreatePending_RaceNoOversell(t *testing.T) {
	pool := testdb.Open(t)
	testdb.Truncate(t, pool)
	const capacity = 3
	const contenders = 10
	f := testdb.SeedBase(t, pool, capacity)
	repo := booking.NewRepository(pool)

	checkIn := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	checkOut := checkIn.AddDate(0, 0, 1) // 1-night stay

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		ok       int
		soldOut  int
		otherErr error
	)
	start := make(chan struct{})

	for i := 0; i < contenders; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			b := &booking.Booking{
				HotelID:           f.HotelID,
				RoomTypeID:        f.RoomTypeID,
				RoomCount:         1,
				GuestEmail:        fmtEmail(i),
				GuestName:         "Race",
				CheckInDate:       checkIn,
				CheckOutDate:      checkOut,
				Currency:          "THB",
				RoomSubtotalCents: 150000,
				TotalCents:        150000,
				Source:            booking.SourceWeb,
			}
			_, err := repo.CreatePending(ctx, b, 10*time.Minute, f.RoomTypeID)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ok++
			case errors.Is(err, booking.ErrNoAvailability):
				soldOut++
			default:
				otherErr = err
			}
		}()
	}
	close(start)
	wg.Wait()

	if otherErr != nil {
		t.Fatalf("unexpected error: %v", otherErr)
	}
	if ok != capacity {
		t.Errorf("successful bookings: want %d, got %d", capacity, ok)
	}
	if ok+soldOut != contenders {
		t.Errorf("total = ok+soldOut: want %d, got %d (ok=%d soldOut=%d)",
			contenders, ok+soldOut, ok, soldOut)
	}

	// Confirm DB invariant: no overlapping date has > capacity active bookings.
	ctx := context.Background()
	var maxSold int
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(room_count),0) FROM bookings
		WHERE room_type_id = $1
		  AND status IN ('pending_payment','confirmed','checked_in')
		  AND $2 >= check_in_date AND $2 < check_out_date
	`, f.RoomTypeID, checkIn).Scan(&maxSold); err != nil {
		t.Fatalf("post-check: %v", err)
	}
	if maxSold > capacity {
		t.Errorf("DB invariant violated: %d sold > %d capacity", maxSold, capacity)
	}
}

// TestRepo_ConfirmThenCheckInOut walks the state machine end-to-end.
func TestRepo_ConfirmThenCheckInOut(t *testing.T) {
	pool := testdb.Open(t)
	testdb.Truncate(t, pool)
	f := testdb.SeedBase(t, pool, 1)
	repo := booking.NewRepository(pool)
	ctx := context.Background()

	b, err := repo.CreatePending(ctx, &booking.Booking{
		HotelID: f.HotelID, RoomTypeID: f.RoomTypeID, RoomCount: 1,
		GuestEmail: "g@example.com", GuestName: "G",
		CheckInDate:  time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		CheckOutDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		Currency:     "THB", RoomSubtotalCents: 150000, TotalCents: 150000,
		Source: booking.SourceWeb,
	}, 10*time.Minute, f.RoomTypeID)
	if err != nil {
		t.Fatalf("CreatePending: %v", err)
	}

	staffID := f.UserID
	confirmed, err := repo.Confirm(ctx, b.ID, "hotel_staff", &staffID)
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	if confirmed.Status != booking.StatusConfirmed {
		t.Errorf("status after confirm: want confirmed, got %s", confirmed.Status)
	}
	if confirmed.ExpiresAt != nil {
		t.Errorf("expires_at should be cleared on confirm: %v", confirmed.ExpiresAt)
	}

	checkedIn, err := repo.CheckIn(ctx, b.ID, &staffID)
	if err != nil {
		t.Fatalf("CheckIn: %v", err)
	}
	if checkedIn.Status != booking.StatusCheckedIn {
		t.Errorf("status: want checked_in, got %s", checkedIn.Status)
	}

	checkedOut, err := repo.CheckOut(ctx, b.ID, &staffID)
	if err != nil {
		t.Fatalf("CheckOut: %v", err)
	}
	if checkedOut.Status != booking.StatusCheckedOut {
		t.Errorf("status: want checked_out, got %s", checkedOut.Status)
	}

	// Audit trail should have created → confirmed → checked_in → checked_out.
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM booking_events WHERE booking_id = $1`, b.ID).Scan(&n); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if n != 4 {
		t.Errorf("events: want 4, got %d", n)
	}

	// Trying to confirm again should fail with InvalidStateTransition.
	if _, err := repo.Confirm(ctx, b.ID, "hotel_staff", &staffID); !errors.Is(err, booking.ErrInvalidStateTransition) {
		t.Errorf("double confirm should ErrInvalidStateTransition, got %v", err)
	}
}

// TestRepo_ExpirePending ensures the worker sweep flips holds past expiry.
func TestRepo_ExpirePending(t *testing.T) {
	pool := testdb.Open(t)
	testdb.Truncate(t, pool)
	f := testdb.SeedBase(t, pool, 1)
	repo := booking.NewRepository(pool)
	ctx := context.Background()

	// Create with negative hold so it's already expired.
	_, err := repo.CreatePending(ctx, &booking.Booking{
		HotelID: f.HotelID, RoomTypeID: f.RoomTypeID, RoomCount: 1,
		GuestEmail: "expired@example.com", GuestName: "X",
		CheckInDate:  time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		CheckOutDate: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
		Currency:     "THB", RoomSubtotalCents: 150000, TotalCents: 150000,
		Source: booking.SourceWeb,
	}, -1*time.Minute, f.RoomTypeID)
	if err != nil {
		t.Fatalf("CreatePending: %v", err)
	}

	n, err := repo.ExpirePending(ctx, time.Now())
	if err != nil {
		t.Fatalf("ExpirePending: %v", err)
	}
	if n != 1 {
		t.Errorf("expired count: want 1, got %d", n)
	}
}

func fmtEmail(i int) string {
	return "race" + string(rune('0'+i)) + "@example.com"
}

// Silence linter for unused import when tests are skipped.
var _ = uuid.New
