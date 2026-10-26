package ipc

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

const MaxMessage = 64 * 1024

var ErrTooLarge = errors.New("message too large")

func Write(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(b) > MaxMessage-1 {
		return ErrTooLarge
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}

func Read(r io.Reader, v any) error {
	br := bufio.NewReaderSize(io.LimitReader(r, MaxMessage+1), 4096)
	var buf bytes.Buffer
	for {
		chunk, err := br.ReadSlice('\n')
		buf.Write(chunk)
		if buf.Len() > MaxMessage {
			return ErrTooLarge
		}
		if err == nil {
			break
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if errors.Is(err, io.EOF) && buf.Len() > 0 {
			break
		}
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimSpace(buf.Bytes())))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing data")
	}
	return nil
}
