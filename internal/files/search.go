package files

// Search across files (PLAN.md 5b.3): names under a folder of /DATA that
// hold every word of the query, case aside. It stays inside /DATA (os.Root),
// never follows links, skips dot-entries unless asked, and stops at a limit
// of results, of entries looked at, and of time, so a big disk can't hold
// the server up.

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"strings"
	"time"
)

// SearchLimits bound one search.
type SearchLimits struct {
	Results int           // 200
	Visit   int           // 200 000 entries looked at
	Time    time.Duration // 4 s
}

// DefaultLimits are the API's.
var DefaultLimits = SearchLimits{Results: 200, Visit: 200_000, Time: 4 * time.Second}

// Found is a search's answer. More is true when a limit stopped it early.
type Found struct {
	Query   string  `json:"query"`
	Path    string  `json:"path"`
	Entries []Entry `json:"entries"`
	More    bool    `json:"more"`
}

var errStop = errors.New("stop")

// Search finds names under p holding every word of q.
func (s *Service) Search(ctx context.Context, p, q string, hidden bool, lim SearchLimits) (Found, error) {
	words := strings.Fields(strings.ToLower(q))
	if len(words) == 0 {
		return Found{}, ErrBadPath
	}
	rel, err := Clean(p)
	if err != nil {
		return Found{}, err
	}
	r, err := s.open()
	if err != nil {
		return Found{}, err
	}
	defer r.Close()
	out := Found{Query: q, Path: apiPath(rel), Entries: []Entry{}}
	deadline := time.Now().Add(lim.Time)
	visited := 0
	err = fs.WalkDir(r.FS(), dotIfEmpty(rel), func(p string, de fs.DirEntry, err error) error {
		if p == "." || p == rel {
			if err != nil {
				return notFound(err)
			}
			return nil
		}
		if err != nil {
			return nil // a folder that can't be read: skip it
		}
		visited++
		if visited > lim.Visit || ctx.Err() != nil || (visited%512 == 0 && time.Now().After(deadline)) {
			out.More = true
			return errStop
		}
		name := de.Name()
		if (!hidden && strings.HasPrefix(name, ".")) || (path.Dir(p) == "." && name == "lost+found") {
			if de.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		low := strings.ToLower(name)
		for _, w := range words {
			if !strings.Contains(low, w) {
				return nil
			}
		}
		out.Entries = append(out.Entries, s.entry(r, p, de))
		if len(out.Entries) >= lim.Results {
			out.More = true
			return errStop
		}
		return nil
	})
	if err != nil && !errors.Is(err, errStop) {
		return Found{}, err
	}
	return out, nil
}
