package output

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkCoordinatorLocalSave(b *testing.B) {
	root := b.TempDir()
	source := filepath.Join(root, "rendered.jpg")
	payload := make([]byte, 1200*1800/3)
	for i := range payload {
		payload[i] = byte((i * 31) % 251)
	}
	if err := os.WriteFile(source, payload, 0o600); err != nil {
		b.Fatal(err)
	}
	coordinator := NewCoordinator(NewLocal(filepath.Join(root, "output")))
	ctx := context.Background()
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		job := Job{
			SessionID:  fmt.Sprintf("%032x", i+1),
			SourcePath: source,
		}
		if _, err := coordinator.Dispatch(ctx, job); err != nil {
			b.Fatal(err)
		}
	}
}
