package files

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"slices"
	"time"
)

// TrashItem is something deleted, waiting in the Trash.
type TrashItem struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	From    string    `json:"from"` // the folder it was in
	Dir     bool      `json:"dir"`
	Size    int64     `json:"size"`
	Deleted time.Time `json:"deleted"`
	Kind    string    `json:"kind"`
}

// Delete moves each path to the Trash: .icedash/trash/<id>/<name>, with
// meta.json saying where it came from.
func (s *Service) Delete(paths []string) error {
	r, err := s.open()
	if err != nil {
		return err
	}
	defer r.Close()
	var rels []string
	for _, p := range paths {
		rel, err := Clean(p)
		if err != nil {
			return err
		}
		if err := s.writable(r, rel); err != nil {
			return err
		}
		if _, err := r.Lstat(rel); err != nil {
			return notFound(err)
		}
		rels = append(rels, rel)
	}
	if err := mkdirPrivate(r, trashDir); err != nil {
		return err
	}
	for _, rel := range rels {
		info, err := r.Lstat(rel)
		if err != nil {
			return notFound(err)
		}
		it := TrashItem{ID: time.Now().UTC().Format("20060102-150405") + "-" + randomID(), Name: path.Base(rel),
			From: apiPath(path.Dir(rel)), Dir: info.IsDir(), Deleted: time.Now().UTC()}
		if it.From == "/." {
			it.From = "/"
		}
		if it.Dir {
			it.Size = sizeOf(r, rel)
		} else {
			it.Size = info.Size()
		}
		slot := path.Join(trashDir, it.ID)
		if err := r.Mkdir(slot, 0o700); err != nil {
			return err
		}
		b, _ := json.Marshal(it)
		if err := r.WriteFile(path.Join(slot, "meta.json"), b, 0o600); err != nil {
			return err
		}
		if err := r.Rename(rel, path.Join(slot, "item")); err != nil {
			_ = r.RemoveAll(slot)
			if isCrossDevice(err) {
				return errors.New("that is on another disk; it can't go to the Trash")
			}
			return err
		}
	}
	return nil
}

func mkdirPrivate(r *os.Root, rel string) error {
	cur := ""
	for _, part := range []string{".icedash", "trash"} {
		cur = path.Join(cur, part)
		if err := r.Mkdir(cur, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
	}
	_ = rel
	return nil
}

// Trash lists the Trash, newest first.
func (s *Service) Trash() ([]TrashItem, error) {
	r, err := s.open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out := []TrashItem{}
	d, err := r.Open(trashDir)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	defer d.Close()
	des, err := d.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	for _, de := range des {
		if it, ok := s.item(r, de.Name()); ok {
			out = append(out, it)
		}
	}
	slices.SortFunc(out, func(a, b TrashItem) int { return b.Deleted.Compare(a.Deleted) })
	return out, nil
}

func (s *Service) item(r *os.Root, id string) (TrashItem, bool) {
	if !ValidName(id) {
		return TrashItem{}, false
	}
	b, err := r.ReadFile(path.Join(trashDir, id, "meta.json"))
	if err != nil {
		return TrashItem{}, false
	}
	var it TrashItem
	if json.Unmarshal(b, &it) != nil || it.ID != id {
		return TrashItem{}, false
	}
	it.Kind = KindOf(it.Name, it.Dir)
	return it, true
}

// Restore puts Trash items back where they were; a taken name gets "(2)",
// and a folder that is gone is made again.
func (s *Service) Restore(ids []string) ([]Entry, error) {
	r, err := s.open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var out []Entry
	for _, id := range ids {
		it, ok := s.item(r, id)
		if !ok {
			return out, ErrNotFound
		}
		dir, err := Clean(it.From)
		if err != nil || !ValidName(it.Name) {
			return out, ErrBadPath
		}
		if err := s.dirWritable(r, dir); err != nil {
			return out, err
		}
		if dir != "" {
			if err := mkdirAll(r, dir); err != nil {
				return out, err
			}
		}
		name := it.Name
		if _, err := r.Lstat(path.Join(dir, name)); err == nil {
			name = freeName(r, dir, name)
		}
		to := path.Join(dir, name)
		if err := r.Rename(path.Join(trashDir, id, "item"), to); err != nil {
			return out, err
		}
		_ = r.RemoveAll(path.Join(trashDir, id))
		e, err := s.stat(r, to)
		if err == nil {
			out = append(out, e)
		}
	}
	return out, nil
}

// Purge deletes Trash items for good; no ids means all of them.
func (s *Service) Purge(ids []string) error {
	r, err := s.open()
	if err != nil {
		return err
	}
	defer r.Close()
	if ids == nil {
		items, err := s.Trash()
		if err != nil {
			return err
		}
		for _, it := range items {
			ids = append(ids, it.ID)
		}
	}
	for _, id := range ids {
		if _, ok := s.item(r, id); !ok {
			return ErrNotFound
		}
		if err := r.RemoveAll(path.Join(trashDir, id)); err != nil {
			return err
		}
	}
	return nil
}
