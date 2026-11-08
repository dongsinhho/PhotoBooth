package delivery

import (
	"errors"
	"fmt"
	"strings"

	stationconfig "github.com/dongsinhho/PhotoBooth/internal/booth/config"
	"github.com/dongsinhho/PhotoBooth/internal/booth/output"
)

// NewDispatcher maps the station's media-delivery policy to an output
// dispatcher. The capture workflow depends only on output.Dispatcher; it does
// not know whether a selected policy is local, LAN or cloud.
//
// Only local delivery is implemented today. Unsupported modes fail closed.
func NewDispatcher(settings stationconfig.MediaDeliverySettings, localOutputDir string) (output.Dispatcher, error) {
	mode := strings.ToLower(strings.TrimSpace(settings.Mode))
	switch mode {
	case stationconfig.MediaDeliveryModeLocal:
		if strings.TrimSpace(localOutputDir) == "" {
			return nil, errors.New("local media delivery requires an output directory")
		}
		return output.NewLocal(localOutputDir), nil
	default:
		return nil, fmt.Errorf("unsupported media delivery mode %q", settings.Mode)
	}
}
