package image

import (
	"bufio"
	"context"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/seantis/roots/pkg/lock"
)

// Store negotiates between the local destination and the remote image,
// optionally caching layers and offering a way to purge the cache.
type Store struct {
	Path string
}

// StoreResult contains the result of a DownloadLayer call
type StoreResult struct {
	Path   string
	Digest string
	Error  error
}

// NewStore returns a new store
func NewStore(folder string) (*Store, error) {

	// ignore path creation errors - if it's serious, we'll know about it later
	_ = os.Mkdir(path.Join(folder, "layers"), 0755)
	_ = os.Mkdir(path.Join(folder, "links"), 0755)

	return &Store{
		Path: folder,
	}, nil
}

// Purge removes all the unused data from the cache
func (s *Store) Purge() error {

	// lock the whole cache
	defer s.lockCache().MustUnlock()

	// load the destination folders and the layers connected to them
	links, err := s.readLinks()
	if err != nil {
		return err
	}

	// keep a list of known layers
	layers := make(map[string]bool)

	for dst, digests := range links {
		_, err := os.Stat(dst)

		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("error reading %s: %v", dst, err)
			}

			// the destination does not exist anymore, remove the link
			if err := os.Remove(s.LinkPath(dst)); err != nil {
				return fmt.Errorf("error removing %s: %v", dst, err)
			}

			continue
		}

		// the destination still exists, add its digest to the known layers
		for _, digest := range digests {
			layers[digest] = true
		}
	}

	// go through all the cached layers and remove the unknown ones
	selector := fmt.Sprintf("%s/layers/*.layer", s.Path)
	cached, err := filepath.Glob(selector)
	if err != nil {
		return fmt.Errorf("error reading %s: %v", selector, err)
	}

	for _, file := range cached {
		digest := strings.TrimSuffix(filepath.Base(file), ".layer")

		if !layers[digest] {
			if err := os.Remove(file); err != nil {
				return fmt.Errorf("error removing %s: %v", file, err)
			}
		}
	}

	return nil
}

// LinkPath returns the path to the link file in the cache
func (s *Store) LinkPath(dst string) string {
	return path.Join(s.Path, "links", fmt.Sprintf("%x.link", md5.Sum([]byte(dst))))
}

// LayerPath returns the path to the layer file in the cache
func (s *Store) LayerPath(digest string) string {
	return path.Join(s.Path, "layers", fmt.Sprintf("%s.layer", digest))
}

// Extract takes a remote, downloads the layers and stores them at dst
func (s *Store) Extract(ctx context.Context, r *Remote, dst string) error {

	// fetch the layers
	layers, err := r.Layers()
	if err != nil {
		return fmt.Errorf("error querying layers for %s: %v", r, err)
	}

	if len(layers) == 0 {
		return fmt.Errorf("no layers found for %s", r)
	}

	// lock the whole destination as well as the cache
	defer s.lockCache().MustUnlock()
	defer s.lockDestination(dst).MustUnlock()

	// ensure the destination is empty
	entries, err := ioutil.ReadDir(dst)
	if err != nil {
		return fmt.Errorf("error extracting to %s: %v", dst, err)
	}

	if len(entries) > 1 {
		return fmt.Errorf("directory %s is not empty", dst)
	}

	// download the layers concurrently
	results := make([]chan *StoreResult, len(layers))
	for i, l := range layers {
		results[i], err = s.downloadLayer(ctx, r, l.Digest)

		if err != nil {
			return fmt.Errorf("error writing %s: %v", l.Digest, err)
		}
	}

	// process the layers in order
	digests := make([]string, len(results))
	dirmodes := make(map[string]os.FileMode)

	for i := range results {
		result := <-results[i]

		if result.Error != nil {
			return fmt.Errorf("error downloading %s: %v", result.Digest, result.Error)
		}

		err := untarLayer(ctx, result.Path, dst, dirmodes)

		if err != nil {
			return fmt.Errorf("error extracting %s: %v", result.Path, err)
		}

		digests[i] = result.Digest
	}

	// set the correct permissions for all directories
	if err := setDirectoryPermissions(dirmodes); err != nil {
		return fmt.Errorf("error setting directory permissions: %v", err)
	}

	// record the destination in the cache
	return s.saveLink(dst, digests)
}

// downloadLayer downloads the given layer into the cache and sends a path
// through the given channel, once the download is complete.
// If the layer was downloaded already, the path will be sent to the channel
// right away.
func (s *Store) downloadLayer(ctx context.Context, r *Remote, digest string) (chan *StoreResult, error) {

	// we need a buffer of 1 so we can send to the channel even if the other
	// side has not yet started listening
	out := make(chan *StoreResult, 1)
	dst := s.LayerPath(digest)

	// if the layer already exists, send it right away
	_, err := os.Stat(dst)
	if err == nil {
		out <- &StoreResult{
			Path:   dst,
			Error:  nil,
			Digest: digest,
		}
		return out, nil
	}

	// otherwise create the file
	w, err := os.Create(dst)
	if err != nil {
		return nil, err
	}

	// then download it in the background
	go func() {
		defer w.Close()
		err := r.DownloadLayer(digest, w)

		out <- &StoreResult{
			Path:   dst,
			Error:  err,
			Digest: digest,
		}
	}()

	return out, nil
}

// saveLink takes a destination and a list of layer digests and records it in
// the cache. The resulting files are used to only Purge what is necessary.
//
// note that this function does not do any locking -> it assumes the cache
// has been locked already
func (s *Store) saveLink(dst string, digests []string) error {

	file := s.LinkPath(dst)
	f, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", file, err)
	}

	// the first line is the header
	if _, err := f.WriteString(fmt.Sprintf("%s\n", dst)); err != nil {
		return fmt.Errorf("error writing %s: %v", file, err)
	}

	//  the other lines are the digests
	for _, digest := range digests {
		if _, err := f.WriteString(fmt.Sprintf("%s\n", digest)); err != nil {
			return fmt.Errorf("error writing %s: %v", file, err)
		}
	}

	return nil
}

// readLinks walks through the stored links and returns a map of the
// destinations and the digests they're associated with
func (s *Store) readLinks() (map[string][]string, error) {
	selector := fmt.Sprintf("%s/links/*.link", s.Path)

	files, err := filepath.Glob(selector)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %v", selector, err)
	}

	links := make(map[string][]string)

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %v", file, err)
		}

		var dst string

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {

			// the first line contains the destination
			if dst == "" {
				dst = scanner.Text()
				continue
			}

			// subsequent lines contain layers
			links[dst] = append(links[dst], scanner.Text())
		}

		// manually close instead of deferring, otherwise files are kept open
		// until the function returns
		f.Close()

		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading %s: %v", file, err)
		}
	}

	return links, nil
}

func (s *Store) lockCache() *lock.InterProcessLock {
	l := &lock.InterProcessLock{Path: path.Join(s.Path, ".lock")}
	l.MustLock()

	return l
}

func (s *Store) lockDestination(dst string) *lock.InterProcessLock {
	l := &lock.InterProcessLock{Path: fmt.Sprintf("%s.lock", dst)}
	l.MustLock()

	return l
}
