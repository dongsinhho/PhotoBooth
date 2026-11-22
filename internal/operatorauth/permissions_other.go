//go:build !windows

package operatorauth

import (
	"errors"
	"os"
)

func validateFilePermissions(info os.FileInfo) error {
	if info.Mode().Perm()&0o077 != 0 {
		return errors.New("operator credential store permissions are too broad; restrict access to the current OS user")
	}
	return nil
}
