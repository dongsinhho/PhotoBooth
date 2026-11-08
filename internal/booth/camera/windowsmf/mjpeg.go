package windowsmf

import "github.com/dongsinhho/PhotoBooth/internal/booth/camera/mjpeg"

// Keep the Windows provider's historical helper name for its regression tests.
// The normalization implementation is shared with Linux V4L2.
func normalizeMJPEG(data []byte) ([]byte, error) {
	return mjpeg.Normalize(data)
}
