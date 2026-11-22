//go:build !windows

package main

import (
	"errors"
)

func (a *App) GetSmbAdminStatus() (SMBAdminStatus, error) {
	if err := a.requireOperator(); err != nil {
		return SMBAdminStatus{}, err
	}
	return SMBAdminStatus{}, errors.New("Windows SMB remote administration is available only in the Windows player")
}

func (a *App) GetSmbAdminInterfaces() ([]SMBAdminInterface, error) {
	if err := a.requireOperator(); err != nil {
		return nil, err
	}
	return nil, errors.New("Windows SMB remote administration is available only in the Windows player")
}

func (a *App) SetSmbAdminEnabled(enabled bool, interfaceAlias, localAddress, remoteAddressText string) error {
	if err := a.requireOperator(); err != nil {
		return err
	}
	return errors.New("Windows SMB remote administration is available only in the Windows player")
}
