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
	"sort"
	"strings"
)

// detect relative paths that try to escape the destination directory
var unsafepath = regexp.MustCompile(`/?\.\./`)

// walkHandler takes a tar.Header and handles it, returning an optional error
type walkHandler func(*tar.Header, *tar.Reader) error

// untarLayer takes an OCI layer and extracts it into a directory, observing
// any whiteouts that might be specified in the layer.
// See: https://github.com/opencontainers/image-spec/blob/master/layer.md
func untarLayer(ctx context.Context, archive, dst string, dirmodes map[string]os.FileMode) error {
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

	reset := func() {
		if _, err := r.Seek(0, 0); err != nil {
			panic(fmt.Errorf("failed to seek %s: %v", archive, err))
		}

		if err := gzr.Reset(r); err != nil {
			panic(fmt.Errorf("failed to reset %s: %v", archive, err))
		}
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

		// create directory structure
		if h.Typeflag == tar.TypeDir {
			file := filepath.Join(dst, h.Name)

			if err := os.MkdirAll(file, 0755); err != nil {
				return fmt.Errorf("error creating directory %s: %v", file, err)
			}

			// store actual file mode of directories to set them later
			dirmodes[file] = os.FileMode(h.Mode)
		}

		return nil
	})

	if err != nil {
		return err
	}

	reset()

	// create all regular files
	err = walkTar(ctx, gzr, func(h *tar.Header, r *tar.Reader) error {

		// skip anything but regular files
		if h.Typeflag != tar.TypeReg {
			return nil
		}

		// skip whiteout files
		if isWhiteoutPath(h.Name) {
			return nil
		}

		// remove the file if it exists
		file := filepath.Join(dst, h.Name)

		if info, err := os.Stat(file); err == nil && !info.IsDir() {
			if err := os.Remove(file); err != nil {
				return fmt.Errorf("error replacing %s: %v", file, err)
			}
		}

		// copy the file
		f, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, os.FileMode(h.Mode))
		if err != nil {
			return fmt.Errorf("error creating %s: %v", file, err)
		}

		if _, err := io.Copy(f, r); err != nil {
			return fmt.Errorf("error copying %s: %v", file, err)
		}

		return f.Close()
	})

	if err != nil {
		return err
	}

	reset()

	// create links
	return walkTar(ctx, gzr, func(h *tar.Header, r *tar.Reader) error {

		// skip anything that isn't a link
		if h.Typeflag != tar.TypeLink && h.Typeflag != tar.TypeSymlink {
			return nil
		}

		new := filepath.Join(dst, h.Name)

		var old string
		if h.Linkname[0] == '.' || !strings.Contains(h.Linkname, "/") {
			old = filepath.Join(filepath.Dir(new), h.Linkname)
		} else {
			old = filepath.Join(dst, h.Linkname)
		}

		// remove the link if it exists
		if info, err := os.Lstat(new); err == nil && !info.IsDir() {
			if err := os.Remove(new); err != nil {
				return fmt.Errorf("error replacing %s: %v", new, err)
			}
		}

		// create hard links
		if h.Typeflag == tar.TypeLink {
			if err := os.Link(old, new); err != nil {
				return fmt.Errorf("error creating hard link %s->%s: %v", new, old, err)
			}
			return nil
		}

		// create symbolic links
		if err := os.Symlink(h.Linkname, new); err != nil {
			return fmt.Errorf("error creating symbolic link %s->%s: %v", new, old, err)
		}

		return nil
	})
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

// setDirectoryPermissions takes a list of directories with file permissions
// and applies the permissions to those files
func setDirectoryPermissions(dirmodes map[string]os.FileMode) error {

	// process directories with longer paths first, to set the permissions
	// of children before setting the permissions of parents
	order := make([]string, 0, len(dirmodes))
	for path := range dirmodes {

		// it's possible that certain paths do not exist anymore, if a
		// whiteout was applied in the process
		if info, err := os.Stat(path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return fmt.Errorf("error accessing %s: %v", path, err)
		} else if !info.IsDir() {
			return fmt.Errorf("not a directory: %s", path)
		}

		order = append(order, path)
	}

	sort.Slice(order, func(j, k int) bool {
		return len(order[j]) > len(order[k])
	})

	for _, path := range order {
		if err := os.Chmod(path, dirmodes[path]); err != nil {
			return fmt.Errorf("error setting %04o on %s: %v", dirmodes[path], path, err)
		}
	}

	return nil
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

			if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
				return err
			}
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
