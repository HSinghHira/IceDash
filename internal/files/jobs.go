package files

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"sync"
	"time"
)

// Job is a move or copy, which can take a while for big folders.
type Job struct {
	ID      string    `json:"id"`
	Kind    string    `json:"kind"` // move, copy
	To      string    `json:"to"`
	Items   int       `json:"items"`
	Total   int64     `json:"total"` // bytes to copy (0 for a plain move)
	Done    int64     `json:"done"`
	State   string    `json:"state"` // running, done, failed, cancelled
	Error   string    `json:"error,omitempty"`
	Started time.Time `json:"started"`
	Ended   time.Time `json:"ended,omitzero"`
}

// job is a Job in progress, with its lock.
type job struct {
	Job
	mu     sync.Mutex
	cancel context.CancelFunc
}

func (j *job) snapshot() Job {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.Job
}

func (j *job) add(n int64) {
	j.mu.Lock()
	j.Done += n
	j.mu.Unlock()
}

// ClashError lists the names already in the target folder.
type ClashError struct{ Names []string }

func (e *ClashError) Error() string { return ErrExists.Error() }
func (e *ClashError) Unwrap() error { return ErrExists }

// Transfer starts moving or copying the items srcs into the folder to.
// The items share one place (a folder's selection); to may be on another
// disk. With ClashAsk it answers a *ClashError before starting when any name
// is taken, so the UI can ask once for all of them.
func (s *Service) Transfer(kind string, srcs []string, to string, clash Clash) (Job, error) {
	if kind != "move" && kind != "copy" {
		return Job{}, errors.New("unknown job")
	}
	if len(srcs) == 0 {
		return Job{}, ErrBadPath
	}
	dst, dir, err := s.resolve(to)
	if err != nil {
		return Job{}, err
	}
	src, _, err := s.resolve(srcs[0])
	if err != nil {
		return Job{}, err
	}
	rd, err := dst.open()
	if err != nil {
		return Job{}, err
	}
	defer rd.Close()
	rs, err := src.open()
	if err != nil {
		return Job{}, err
	}
	defer rs.Close()
	same := src == dst
	if err := s.dirWritable(rd, dir); err != nil {
		return Job{}, err
	}
	if info, err := rd.Stat(dotIfEmpty(dir)); err != nil {
		return Job{}, notFound(err)
	} else if !info.IsDir() {
		return Job{}, ErrNotDir
	}
	var rels []string
	var taken []string
	for _, p := range srcs {
		pl, rel, err := s.resolve(p)
		if err != nil {
			return Job{}, err
		}
		if pl != src {
			return Job{}, ErrBadPath
		}
		if rel == "" {
			return Job{}, ErrProtected
		}
		if kind == "move" {
			if err := s.writable(rs, rel); err != nil {
				return Job{}, err
			}
		}
		if _, err := rs.Lstat(rel); err != nil {
			return Job{}, notFound(err)
		}
		if same && (dir == rel || strings.HasPrefix(dir+"/", rel+"/")) {
			return Job{}, ErrIntoSelf
		}
		target := path.Join(dir, path.Base(rel))
		if Protected(target) {
			return Job{}, ErrProtected
		}
		if same && (path.Dir(rel) == dir || (dir == "" && !strings.Contains(rel, "/"))) {
			if kind == "move" {
				continue // already there
			}
		}
		if _, err := rd.Lstat(target); err == nil {
			taken = append(taken, path.Base(rel))
		}
		rels = append(rels, rel)
	}
	if len(taken) > 0 && clash == ClashAsk {
		return Job{}, &ClashError{Names: taken}
	}

	var total int64
	if kind == "copy" || !same {
		for _, rel := range rels {
			total += sizeOf(rs, rel)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	j := &job{Job: Job{ID: randomID(), Kind: kind, To: dst.api(dir), Items: len(rels), Total: total,
		State: "running", Started: time.Now().UTC()}, cancel: cancel}
	s.mu.Lock()
	if s.jobs == nil {
		s.jobs = map[string]*job{}
	}
	s.gcJobs()
	s.jobs[j.ID] = j
	s.mu.Unlock()
	go s.run(ctx, j, src, rels, dst, dir, clash)
	return j.snapshot(), nil
}

func (s *Service) gcJobs() {
	for id, j := range s.jobs {
		st := j.snapshot()
		if st.State != "running" && time.Since(st.Ended) > time.Hour {
			delete(s.jobs, id)
		}
	}
}

func (s *Service) run(ctx context.Context, j *job, src place, rels []string, dst place, dir string, clash Clash) {
	err := func() error {
		rs, err := src.open()
		if err != nil {
			return err
		}
		defer rs.Close()
		rd := rs
		if dst != src {
			if rd, err = dst.open(); err != nil {
				return err
			}
			defer rd.Close()
		}
		for _, rel := range rels {
			name := path.Base(rel)
			target := path.Join(dir, name)
			if info, err := rd.Lstat(target); err == nil {
				switch {
				case clash == ClashSkip:
					continue
				case clash == ClashKeep:
					target = path.Join(dir, freeName(rd, dir, name))
				case clash == ClashReplace && !info.IsDir():
					if err := rd.Remove(target); err != nil {
						return err
					}
				default:
					return ErrExists
				}
			}
			if j.Kind == "move" {
				if dst == src {
					err := rs.Rename(rel, target)
					if err == nil {
						continue
					}
					if !isCrossDevice(err) {
						return err
					}
				}
				// Another disk: copy, then remove the original.
				if err := copyTree(ctx, rs, rel, rd, target, j); err != nil {
					return err
				}
				if err := rs.RemoveAll(rel); err != nil {
					return err
				}
				continue
			}
			if err := copyTree(ctx, rs, rel, rd, target, j); err != nil {
				return err
			}
		}
		return nil
	}()
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Ended = time.Now().UTC()
	switch {
	case errors.Is(err, context.Canceled):
		j.State = "cancelled"
	case err != nil:
		j.State, j.Error = "failed", notFound(err).Error()
	default:
		j.State = "done"
	}
}

func isCrossDevice(err error) bool {
	return err != nil && strings.Contains(err.Error(), "cross-device")
}

// sizeOf adds up the regular files under rel.
func sizeOf(r *os.Root, rel string) int64 {
	var n int64
	_ = fs.WalkDir(r.FS(), rel, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

// copyTree copies a file or folder from rs to rd (the same root, or another
// disk's), keeping permissions; links are copied as links. A cancelled copy
// removes what it made.
func copyTree(ctx context.Context, rs *os.Root, src string, rd *os.Root, dst string, j *job) (err error) {
	defer func() {
		if err != nil {
			_ = rd.RemoveAll(dst)
		}
	}()
	return fs.WalkDir(rs.FS(), src, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		out := path.Join(dst, strings.TrimPrefix(strings.TrimPrefix(p, src), "/"))
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			if err := rd.Mkdir(out, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
				return err
			}
			_ = rd.Chmod(out, info.Mode().Perm()) // vfat and exfat can't
			return nil
		case d.Type()&fs.ModeSymlink != 0:
			target, err := rs.Readlink(p)
			if err != nil {
				return err
			}
			return rd.Symlink(target, out)
		case d.Type().IsRegular():
			return copyFile(ctx, rs, p, rd, out, info.Mode().Perm(), j)
		}
		return nil // sockets, devices: left out
	})
}

func copyFile(ctx context.Context, rs *os.Root, src string, rd *os.Root, dst string, perm fs.FileMode, j *job) error {
	in, err := rs.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := rd.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	buf := make([]byte, 1<<20)
	for {
		if ctx.Err() != nil {
			out.Close()
			return ctx.Err()
		}
		n, rerr := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				return werr
			}
			j.add(int64(n))
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			out.Close()
			return rerr
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	_ = rd.Chmod(dst, perm) // vfat and exfat can't
	return nil
}

// Jobs lists the running jobs and those that ended in the last hour.
func (s *Service) Jobs() []Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Job{}
	for _, j := range s.jobs {
		st := j.snapshot()
		if st.State == "running" || time.Since(st.Ended) < time.Hour {
			out = append(out, st)
		}
	}
	slices.SortFunc(out, func(a, b Job) int { return b.Started.Compare(a.Started) })
	return out
}

// Cancel stops a running job.
func (s *Service) Cancel(id string) bool {
	s.mu.Lock()
	j, ok := s.jobs[id]
	s.mu.Unlock()
	if !ok {
		return false
	}
	j.cancel()
	return true
}
