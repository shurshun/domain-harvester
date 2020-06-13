package types

import (
	"time"
)

type WhoisHarverster interface {
	GetDomainData(domain string) (*WhoisData, error)
	GetExternalRequestsCnt() uint64
}

type WhoisData struct {
	Domain      string
	ExpiryDays  float64
	LastUpdated time.Time
	Error       string
}
