package httpapi

import (
	"clipflow/internal/service"
	"io"
	"os"
)

func osTemp(r io.Reader) (*os.File, error) {
	f, err := os.CreateTemp("", "clipflow-request-*.mp4")
	if err != nil {
		return nil, err
	}
	n, err := io.Copy(f, io.LimitReader(r, service.MaxUpload+1))
	if n > service.MaxUpload {
		err = service.ErrTooLarge
	}
	if err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	_, err = f.Seek(0, 0)
	if err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	return f, nil
}
func removeTemp(f *os.File) { os.Remove(f.Name()) }
