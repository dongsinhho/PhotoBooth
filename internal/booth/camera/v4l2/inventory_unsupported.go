//go:build !linux

package v4l2

import "runtime"

// Inventory reports /dev/video* nodes and access metadata without opening or
// streaming from any camera device.
func Inventory() InventoryReport {
	return InventoryReport{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		Devices:      []DeviceInventory{},
		Warnings:     []string{"V4L2 inventory is only available on Linux"},
	}
}
