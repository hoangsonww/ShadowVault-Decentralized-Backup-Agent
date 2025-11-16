package compression

import (
	"crypto/rand"
	"testing"
)

func BenchmarkZstdCompression(b *testing.B) {
	data := make([]byte, 1024*1024) // 1MB
	rand.Read(data)

	compressor, err := NewCompressor(Zstd, 3)
	if err != nil {
		b.Fatal(err)
	}
	defer compressor.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.Compress(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGzipCompression(b *testing.B) {
	data := make([]byte, 1024*1024) // 1MB
	rand.Read(data)

	compressor, err := NewCompressor(Gzip, 6)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.Compress(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompressionLevels(b *testing.B) {
	data := make([]byte, 1024*1024) // 1MB
	rand.Read(data)

	levels := []int{1, 3, 6, 9}

	for _, level := range levels {
		b.Run("Zstd-Level-"+string(rune(level+'0')), func(b *testing.B) {
			compressor, err := NewCompressor(Zstd, level)
			if err != nil {
				b.Fatal(err)
			}
			defer compressor.Close()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := compressor.Compress(data)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkDecompression(b *testing.B) {
	data := make([]byte, 1024*1024) // 1MB
	rand.Read(data)

	compressor, err := NewCompressor(Zstd, 3)
	if err != nil {
		b.Fatal(err)
	}
	defer compressor.Close()

	compressed, err := compressor.Compress(data)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := compressor.Decompress(compressed)
		if err != nil {
			b.Fatal(err)
		}
	}
}
