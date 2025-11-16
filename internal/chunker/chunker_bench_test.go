package chunker

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func BenchmarkChunking(b *testing.B) {
	// Generate test data
	data := make([]byte, 1024*1024) // 1MB
	rand.Read(data)

	chunker := New(2048, 65536, 8192)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		chunks := chunker.Split(reader)

		// Consume all chunks
		for range chunks {
		}
	}
}

func BenchmarkChunkingSmallFile(b *testing.B) {
	data := make([]byte, 10*1024) // 10KB
	rand.Read(data)

	chunker := New(2048, 65536, 8192)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		chunks := chunker.Split(reader)
		for range chunks {
		}
	}
}

func BenchmarkChunkingLargeFile(b *testing.B) {
	data := make([]byte, 10*1024*1024) // 10MB
	rand.Read(data)

	chunker := New(2048, 65536, 8192)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		chunks := chunker.Split(reader)
		for range chunks {
		}
	}
}

func BenchmarkChunkingDifferentSizes(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
		{"10MB", 10 * 1024 * 1024},
	}

	for _, tc := range sizes {
		b.Run(tc.name, func(b *testing.B) {
			data := make([]byte, tc.size)
			rand.Read(data)

			chunker := New(2048, 65536, 8192)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				reader := bytes.NewReader(data)
				chunks := chunker.Split(reader)
				for range chunks {
				}
			}
		})
	}
}
