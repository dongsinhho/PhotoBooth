package mf

import "github.com/Kirizu-Official/windows-camera-go/windows/consts"

// controlSetResultError reports every non-S_OK HRESULT as a failed control
// write. In particular, ERROR_NOT_FOUND must not be treated as success:
// a Set call that did not find/apply a property must not look successful.
func controlSetResultError(result uintptr) error {
	if result == uintptr(consts.S_OK) {
		return nil
	}
	return consts.HResultError{Code: consts.HRESULT(result)}
}
