package main

import (
	"errors"
	"fmt"
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
			fmt.Printf("Resettings dev input")
			if _, err := r.input.Seek(0, io.SeekStart); err != nil {
				panic(err)
			} else {
				return r.input.Read(p)
			}
		}
		panic(err)
	} else {
		return n, nil
	}
}

func (r DevBinReader) Seek(offset int64, whence int) (int64, error) {
	return r.input.Seek(offset, whence)
}
