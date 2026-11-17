//go:build !windows

package main

import (
	"errors"
)

func (a *App) GetSmbAdminStatus() (SMBAdminStatus, error) {
	return SMBAdminStatus{}, errors.New("Windows SMB remote administration is available only in the Windows player")
}

func (a *App) GetSmbAdminInterfaces() ([]SMBAdminInterface, error) {
	return nil, errors.New("Windows SMB remote administration is available only in the Windows player")
}

func (a *App) SetSmbAdminEnabled(enabled bool, interfaceAlias, localAddress, remoteAddressText string) error {
	return errors.New("Windows SMB remote administration is available only in the Windows player")
}
