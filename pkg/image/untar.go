package image

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/codeclysm/extract"
)

// detect relative paths that try to escape the destination directory
var unsafepath = regexp.MustCompile(`/?\.\./`)

// walkHandler takes a tar.Header and handles it, returning an optional error
type walkHandler func(*tar.Header, *tar.Reader) error

// untarLayer takes an OCI layer and extracts it into a directory, observing
// any whiteouts that might be specified in the layer.
// See: https://github.com/opencontainers/image-spec/blob/master/layer.md
func untarLayer(ctx context.Context, archive, dst string) error {
	r, err := os.Open(archive)
	if err == nil {
		defer r.Close()
	} else {
		return err
	}

	gzr, err := gzip.NewReader(r)
	if err == nil {
		defer gzr.Close()
	} else {
		return err
	}

	// pre-process the archive
	err = walkTar(ctx, gzr, func(h *tar.Header, r *tar.Reader) error {

		// apply whiteout files
		if isWhiteoutPath(h.Name) {
			if err := applyWhiteout(dst, h.Name); err != nil {
				return err
			}
		}

		// detect unsafe filenames and stop everything if found
		if unsafepath.MatchString(h.Name) {
			return fmt.Errorf("refusing to extract unsafe path: %s", h.Name)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// then extract all non-whiteout files
	r.Seek(0, 0)
	gzr.Reset(r)

	extract.Tar(ctx, gzr, dst, func(name string) string {
		if isWhiteoutPath(name) {
			return "" // file will be skipped
		}

		return name
	})

	if err != nil {
		return err
	}

	return nil
}

// walkTar takes a gzip.Reader and calls a handler function
func walkTar(ctx context.Context, gzr *gzip.Reader, handler walkHandler) error {
	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("failed to walk tar: %v", err)
			}
			return nil
		}

		select {
		case <-ctx.Done():
			return errors.New("interrupted")
		default:
			err = handler(header, tr)

			if err != nil {
				return err
			}
		}
	}
}

// applyWhiteout takes a destination and a relative whiteout path and applies it
func applyWhiteout(dst, whiteout string) error {
	if strings.HasSuffix(whiteout, ".wh..wh..opq") {
		return applyOpaqueWhiteout(dst, whiteout)
	}

	return applySimpleWhiteout(dst, whiteout)
}

func applyOpaqueWhiteout(dst, whiteout string) error {
	base := path.Join(dst, filepath.Dir(whiteout))

	f, err := os.Open(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}
	defer f.Close()

	const buffer = 10

	for {
		lst, err := f.Readdir(buffer)

		if err == io.EOF {
			return nil
		}

		for _, info := range lst {
			file := path.Join(base, info.Name())

			if info.IsDir() {
				err = os.RemoveAll(file)

				if err != nil {
					return err
				}
			}

			err = os.Remove(file)
		}
	}
}

func applySimpleWhiteout(dst, whiteout string) error {
	file := path.Join(dst, filepath.Dir(whiteout), filepath.Base(whiteout)[4:])
	info, err := os.Stat(file)

	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	if info.IsDir() {
		return os.RemoveAll(file)
	}

	return os.Remove(file)
}

func isWhiteoutPath(p string) bool {
	return strings.HasPrefix(filepath.Base(p), ".wh.")
}
