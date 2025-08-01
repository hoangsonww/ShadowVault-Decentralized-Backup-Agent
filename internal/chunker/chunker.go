package chunker

import (
	"hash"
	"hash/fnv"
	"io"
)

// Simple content-defined chunking using rolling hash (simplified).
// Breaks when lower bits of rolling hash hit a pattern to get average size.

type Chunker struct {
	r      io.Reader
	min, max, avg int
	window []byte
}

const (
	defaultMaskBits = 13 // ~8192 average chunk size
)

func New(r io.Reader, min, max, avg int) *Chunker {
	return &Chunker{
		r:      r,
		min:    min,
		max:    max,
		avg:    avg,
		window: make([]byte, 0, max),
	}
}

func boundary(hash uint32, mask uint32) bool {
	return (hash & mask) == 0
}

func (c *Chunker) Next() ([]byte, error) {
	buf := make([]byte, c.max)
	n, err := c.r.Read(buf)
	if n == 0 && err != nil {
		return nil, err
	}
	data := buf[:n]
	// rolling scan to find boundary
	var h hash.Hash32 = fnv.New32a()
	chunkEnd := len(data)
	mask := uint32((1 << (defaultMaskBits)) - 1)
	if chunkEnd > c.max {
		chunkEnd = c.max
	}
	for i := 0; i < len(data); i++ {
		h.Write([]byte{data[i]})
		if i >= c.min && boundary(h.Sum32(), mask) {
			chunkEnd = i + 1
			break
		}
		if i >= c.max-1 {
			chunkEnd = c.max
			break
		}
	}
	return data[:chunkEnd], nil
}
