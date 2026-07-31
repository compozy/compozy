package securehttp

import "io"

type limitedReadCloser struct {
	io.ReadCloser
	remaining int64
	overLimit bool
}

func (r *limitedReadCloser) Read(buffer []byte) (int, error) {
	if r.overLimit {
		return 0, ErrResponseBodyTooLarge
	}
	if r.remaining == 0 {
		var oneByte [1]byte
		read, err := r.ReadCloser.Read(oneByte[:])
		if read > 0 {
			r.overLimit = true
			return 0, ErrResponseBodyTooLarge
		}
		return 0, err
	}
	if int64(len(buffer)) > r.remaining {
		buffer = buffer[:r.remaining]
	}
	read, err := r.ReadCloser.Read(buffer)
	r.remaining -= int64(read)
	return read, err
}
