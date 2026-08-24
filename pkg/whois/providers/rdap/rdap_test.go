package rdap

import (
	"testing"
	"time"

	"github.com/openrdap/rdap"
)

func TestExpirationDate(t *testing.T) {
	t.Run("finds the expiration event among others", func(t *testing.T) {
		d := &rdap.Domain{
			Events: []rdap.Event{
				{Action: "registration", Date: "1997-09-15T04:00:00Z"},
				{Action: "expiration", Date: "2030-09-14T04:00:00Z"},
				{Action: "last changed", Date: "2019-09-09T15:39:04Z"},
			},
		}

		got, err := expirationDate(d)
		if err != nil {
			t.Fatalf("expirationDate() error = %v", err)
		}

		want := time.Date(2030, 9, 14, 4, 0, 0, 0, time.UTC)
		if !got.Equal(want) {
			t.Errorf("expirationDate() = %v, want %v", got, want)
		}
	})

	t.Run("errors when there is no expiration event", func(t *testing.T) {
		d := &rdap.Domain{
			Events: []rdap.Event{
				{Action: "registration", Date: "1997-09-15T04:00:00Z"},
			},
		}

		if _, err := expirationDate(d); err == nil {
			t.Fatal("expirationDate() error = nil, want an error")
		}
	})

	t.Run("errors on an unparsable date", func(t *testing.T) {
		d := &rdap.Domain{
			Events: []rdap.Event{
				{Action: "expiration", Date: "not-a-date"},
			},
		}

		if _, err := expirationDate(d); err == nil {
			t.Fatal("expirationDate() error = nil, want an error")
		}
	})
}

func TestNew(t *testing.T) {
	// New must apply DefaultTimeout when given a non-positive value, rather
	// than leaving the client to block indefinitely.
	if p := New(0); p == nil || p.client == nil || p.client.HTTP.Timeout != DefaultTimeout {
		t.Fatal("New(0) did not apply DefaultTimeout")
	}

	if p := New(-time.Second); p == nil || p.client == nil || p.client.HTTP.Timeout != DefaultTimeout {
		t.Fatal("New(negative) did not apply DefaultTimeout")
	}

	if p := New(5 * time.Second); p.client.HTTP.Timeout != 5*time.Second {
		t.Errorf("New(5s) client timeout = %v, want 5s", p.client.HTTP.Timeout)
	}
}
