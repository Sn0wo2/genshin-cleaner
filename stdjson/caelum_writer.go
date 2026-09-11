package stdjson

import (
	"bytes"
	"encoding/json"
	"io"
)

type Writer struct {
	dst    io.Writer
	pretty bool
}

func (w *Writer) Write(p []byte) (int, error) {
	if !w.pretty {
		return w.dst.Write(p)
	}
	var out bytes.Buffer
	if err := json.Indent(&out, bytes.TrimSuffix(p, []byte{'\n'}), "", "  "); err != nil {
		return w.dst.Write(p)
	}
	out.WriteByte('\n')
	if _, err := w.dst.Write(out.Bytes()); err != nil {
		return 0, err
	}
	return len(p), nil
}
