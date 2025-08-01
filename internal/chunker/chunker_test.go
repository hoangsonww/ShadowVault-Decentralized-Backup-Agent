package chunker_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/hoangsonww/backupagent/internal/chunker"
)

func TestChunkerBasic(t *testing.T) {
	data := strings.Repeat("a", 50000) // large data
	r := strings.NewReader(data)
	ch := chunker.New(r, 2048, 8192, 4096)
	var total int
	for {
		b, err := ch.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("chunker error: %v", err)
		}
		if len(b) == 0 {
			t.Fatalf("got empty chunk")
		}
		total += len(b)
	}
	if total != len(data) {
		t.Fatalf("expected total %d got %d", len(data), total)
	}
}

func TestChunkerSmall(t *testing.T) {
	data := "short"
	r := strings.NewReader(data)
	ch := chunker.New(r, 1, 10, 5)
	b, err := ch.Next()
	if err != nil && err != io.EOF {
		t.Fatalf("error: %v", err)
	}
	if string(b) != data {
		t.Fatalf("expected %s got %s", data, string(b))
	}
}

func TestChunkerEOF(t *testing.T) {
	r := bytes.NewBuffer(nil)
	ch := chunker.New(r, 1, 10, 5)
	_, err := ch.Next()
	if err == nil {
		t.Fatalf("expected EOF on empty reader")
	}
}
