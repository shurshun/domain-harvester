package local

import (
	"domain-harvester/pkg/whois/types"
	"fmt"
	whois "github.com/shift/whois"
	"math"
	"regexp"
	"strings"
	// "sync"
	"time"
)

var formats = []string{
	"2006-01-02",
	"2006-01-02T15:04:05Z",
	"02-Jan-2006",
	"2006.01.02",
	"Mon Jan 2 15:04:05 MST 2006",
	"02/01/2006",
	"2006-01-02 15:04:05 MST",
	"2006/01/02",
	"Mon Jan 2006 15:04:05",
}

type WhoisProvider struct {
	regexpTime *regexp.Regexp
	// mutex      sync.RWMutex
	externalRequests uint64
}

func withError(wd *types.WhoisData, err error) (*types.WhoisData, error) {
	wd.Error = err.Error()

	return wd, err
}

func (wp *WhoisProvider) Init() {
	wp.regexpTime, _ = regexp.Compile(`(Registry Expiry Date|paid-till|Expiration Date|Expiry.*): (.*)`)
	wp.externalRequests = 0
}

func (wp *WhoisProvider) GetExternalRequestsCnt() uint64 {
	return wp.externalRequests
}

func (wp *WhoisProvider) GetDomainData(domain string) (*types.WhoisData, error) {
	wd := &types.WhoisData{Domain: domain, ExpiryDays: 0, LastUpdated: time.Now(), Error: ""}

	wp.externalRequests++

	req, err := whois.NewRequest(domain)
	if err != nil {
		return withError(wd, err)
	}

	var res *whois.Response

	res, err = whois.DefaultClient.Fetch(req)
	if err != nil {
		return withError(wd, err)
	}
	result := wp.regexpTime.FindStringSubmatch(res.String())

	if len(result) < 2 {
		return withError(wd, fmt.Errorf("Don't know how to parse data: %s", res.String()))
	}

	rawDate := strings.TrimSpace(result[2])

	for _, format := range formats {
		if date, err := time.Parse(format, rawDate); err == nil {
			wd.ExpiryDays = math.Floor(time.Until(date).Hours() / 24)

			return wd, nil
		}
	}

	return withError(wd, fmt.Errorf("Unable to parse date: %s", rawDate))
}
