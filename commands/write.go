// commands/write.go - Writing output files

package commands

import (
	"os"
	"path/filepath"
)

// writeFileAtomic writes data to path with the given mode. It writes a
// temporary file in the same directory, which is created with mode 0600,
// and renames it into place, so path never holds a partial file or looser
// permissions (R-SEC-1, R-SEC-2).
func writeFileAtomic(path string, data []byte, mode os.FileMode) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()

	if err = f.Chmod(mode); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
