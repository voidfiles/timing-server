package main

import (
	"errors"
	"io"
)

type DevBinReader struct {
	input io.ReadSeekCloser
}

func (r DevBinReader) Close() error {
	return r.input.Close()
}

func (r DevBinReader) Read(p []byte) (n int, err error) {
	if n, err := r.input.Read(p); err != nil {
		if errors.Is(err, io.EOF) {
			if _, err := r.input.Seek(0, io.SeekStart); err != nil {
				return r.input.Read(p)
			} else {
				return 0, err
			}
		}
	} else {
		return n, nil
	}

	return 0, nil
}

func (r DevBinReader) Seek(offset int64, whence int) (int64, error) {
	return r.input.Seek(offset, whence)
}
