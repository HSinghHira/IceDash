// Package places finds a town by name for the weather location (community
// feedback, 2026-11-20: the time zone's city can be far from the user, as
// Los Angeles is from north of Seattle). It asks Open-Meteo's geocoding,
// the service the Weather widget already uses, from the server, so the
// IceDash page itself still talks to nobody else.
package places

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

// Place is one match.
type Place struct {
	Name    string  `json:"name"`
	Region  string  `json:"region,omitempty"` // state, province
	Country string  `json:"country,omitempty"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
}

// Label is how the place reads in a list: "Everett, Washington, United States".
func (p Place) Label() string {
	parts := []string{p.Name}
	for _, s := range []string{p.Region, p.Country} {
		if s != "" && s != p.Name {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}

// Search is Open-Meteo's geocoding.
type Search struct {
	HTTP *http.Client
	Base string // default https://geocoding-api.open-meteo.com
}

// ErrQuery is a name too short or too long to look up.
var ErrQuery = errors.New("type at least 2 letters of a town or city")

// Find returns up to 8 matches for a name, in the language given ("en", "de"…).
func (s *Search) Find(ctx context.Context, name, lang string) ([]Place, error) {
	name = strings.TrimSpace(name)
	// "Everett, WA": Open-Meteo matches the name alone; the list shows regions.
	if i := strings.IndexByte(name, ','); i > 0 {
		name = strings.TrimSpace(name[:i])
	}
	if n := utf8.RuneCountInString(name); n < 2 || n > 100 {
		return nil, ErrQuery
	}
	base := s.Base
	if base == "" {
		base = "https://geocoding-api.open-meteo.com"
	}
	if lang == "" {
		lang = "en"
	}
	q := url.Values{"name": {name}, "count": {"8"}, "language": {lang}, "format": {"json"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/search?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	c := s.HTTP
	if c == nil {
		c = http.DefaultClient
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("can't reach the place search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the place search answered %s", resp.Status)
	}
	var body struct {
		Results []struct {
			Name      string  `json:"name"`
			Admin1    string  `json:"admin1"`
			Country   string  `json:"country"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("the place search sent something unreadable: %w", err)
	}
	out := []Place{}
	for _, r := range body.Results {
		out = append(out, Place{Name: r.Name, Region: r.Admin1, Country: r.Country, Lat: r.Latitude, Lon: r.Longitude})
	}
	return out, nil
}
