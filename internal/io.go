package internal

import (
	"io"
)

type readerWithHeader struct {
	r      io.Reader
	header []byte
}

func (rh *readerWithHeader) Read(p []byte) (n int, err error) {
	if rh.header != nil {
		num := copy(p, rh.header)
		if num < len(rh.header) {
			rh.header = rh.header[num:]
			return num, nil
		}
		rh.header = nil
		n, err = rh.r.Read(p[num:])
		n += num
		return
	}
	return rh.r.Read(p)
}

func ReaderWithHeader(reader io.Reader, header []byte) io.Reader {
	h := make([]byte, len(header))
	copy(h, header)
	return &readerWithHeader{reader, h}
}
