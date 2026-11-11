package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Repo is where IceDash's releases come from.
const Repo = "HSinghHira/IceDash"

// Errors with a stable meaning for the UI.
var (
	ErrNoRelease   = errors.New("no release published yet")
	ErrRateLimited = errors.New("GitHub's limit for update checks is reached; try again in an hour")
	ErrUnreachable = errors.New("can't reach GitHub")
)

// Release is the part of GitHub's release JSON IceDash uses.
type Release struct {
	Tag    string  `json:"tag_name"`
	Body   string  `json:"body"`
	Assets []Asset `json:"assets"`
}

// Asset is one file of a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// GitHub asks the releases API. IceDash calls GitHub only for this (ground rule 8).
type GitHub struct {
	API  string // https://api.github.com
	Repo string
	HTTP *http.Client
}

// Latest returns the newest published release.
func (g GitHub) Latest(ctx context.Context) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(g.API, "/")+"/repos/"+g.Repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "IceDash-updater")
	resp, err := g.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNoRelease
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return nil, ErrRateLimited
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var rel Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("GitHub's answer: %w", err)
	}
	if rel.Tag == "" {
		return nil, errors.New("GitHub's answer has no tag")
	}
	return &rel, nil
}

// AllowedURL accepts only HTTPS downloads from GitHub's own hosts, for the
// first URL and every redirect (release assets redirect to
// release-assets.githubusercontent.com, measured on the box).
func AllowedURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	h := strings.ToLower(u.Hostname())
	if u.Scheme == "https" && (h == "github.com" || strings.HasSuffix(h, ".githubusercontent.com")) {
		return nil
	}
	return fmt.Errorf("refusing to download from %s: not GitHub", u.Redacted())
}
