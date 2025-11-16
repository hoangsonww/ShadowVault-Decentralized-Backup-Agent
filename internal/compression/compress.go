package compression

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

// Type represents the compression algorithm
type Type int

const (
	None Type = iota
	Gzip
	Zstd
)

// Compressor handles data compression
type Compressor struct {
	compressionType Type
	level           int
	zstdEncoder     *zstd.Encoder
}

// NewCompressor creates a new compressor
func NewCompressor(t Type, level int) (*Compressor, error) {
	c := &Compressor{
		compressionType: t,
		level:           level,
	}

	// Pre-create zstd encoder for better performance
	if t == Zstd {
		var err error
		c.zstdEncoder, err = zstd.NewWriter(nil,
			zstd.WithEncoderLevel(zstd.EncoderLevelFromZstd(level)))
		if err != nil {
			return nil, fmt.Errorf("failed to create zstd encoder: %w", err)
		}
	}

	return c, nil
}

// Compress compresses data
func (c *Compressor) Compress(data []byte) ([]byte, error) {
	switch c.compressionType {
	case None:
		return data, nil
	case Gzip:
		return c.compressGzip(data)
	case Zstd:
		return c.compressZstd(data)
	default:
		return nil, fmt.Errorf("unsupported compression type: %d", c.compressionType)
	}
}

// Decompress decompresses data
func (c *Compressor) Decompress(data []byte) ([]byte, error) {
	switch c.compressionType {
	case None:
		return data, nil
	case Gzip:
		return c.decompressGzip(data)
	case Zstd:
		return c.decompressZstd(data)
	default:
		return nil, fmt.Errorf("unsupported compression type: %d", c.compressionType)
	}
}

// compressGzip compresses data using gzip
func (c *Compressor) compressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, c.level)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip writer: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		w.Close()
		return nil, fmt.Errorf("failed to write compressed data: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// decompressGzip decompresses gzip data
func (c *Compressor) decompressGzip(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer r.Close()

	decompressed, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read decompressed data: %w", err)
	}

	return decompressed, nil
}

// compressZstd compresses data using zstd
func (c *Compressor) compressZstd(data []byte) ([]byte, error) {
	return c.zstdEncoder.EncodeAll(data, make([]byte, 0, len(data))), nil
}

// decompressZstd decompresses zstd data
func (c *Compressor) decompressZstd(data []byte) ([]byte, error) {
	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd decoder: %w", err)
	}
	defer decoder.Close()

	decompressed, err := decoder.DecodeAll(data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress zstd data: %w", err)
	}

	return decompressed, nil
}

// Close releases resources
func (c *Compressor) Close() error {
	if c.zstdEncoder != nil {
		return c.zstdEncoder.Close()
	}
	return nil
}

// DefaultCompressor returns a default zstd compressor
func DefaultCompressor() (*Compressor, error) {
	return NewCompressor(Zstd, 3) // Level 3 is a good balance of speed and compression
}
