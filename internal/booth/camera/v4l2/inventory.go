package v4l2

// DeviceInventory is a read-only snapshot of Linux V4L2 device nodes.
// Inventory never opens a camera node or starts a stream.
type DeviceInventory struct {
	Path      string `json:"path"`
	Name      string `json:"name,omitempty"`
	OwnerID   uint32 `json:"owner_id"`
	GroupID   uint32 `json:"group_id"`
	GroupName string `json:"group_name,omitempty"`
	Mode      string `json:"mode"`
	ReadWrite bool   `json:"read_write_access"`
}

type InventoryReport struct {
	OS                       string            `json:"os"`
	Architecture             string            `json:"architecture"`
	Kernel                   string            `json:"kernel,omitempty"`
	EffectiveUID             int               `json:"effective_uid"`
	EffectiveGID             int               `json:"effective_gid"`
	Groups                   []string          `json:"groups,omitempty"`
	UVCDriverLoaded          bool              `json:"uvc_driver_loaded"`
	UVCDriverModuleAvailable bool              `json:"uvc_driver_module_available"`
	InVideoGroup             bool              `json:"in_video_group"`
	Video4LinuxSysfs         bool              `json:"video4linux_sysfs_present"`
	Devices                  []DeviceInventory `json:"devices"`
	Warnings                 []string          `json:"warnings,omitempty"`
}
