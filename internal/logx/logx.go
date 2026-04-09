package logx

import (
	"io"
	"os"
	"path/filepath"
	"time"

	klog "github.com/go-kratos/kratos/v2/log"
)

func newLogger(w io.Writer) klog.Logger {
	return klog.With(klog.NewStdLogger(w), "ts", klog.Timestamp(time.RFC3339))
}

// New 返回 Kratos Helper：filePath 非空时同时写入 stdout 与文件；空则仅 stdout。
func New(filePath string) (*klog.Helper, func() error, error) {
	if filePath == "" {
		l := newLogger(os.Stdout)
		return klog.NewHelper(l), func() error { return nil }, nil
	}
	dir := filepath.Dir(filePath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, nil, err
		}
	}
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, nil, err
	}
	mw := io.MultiWriter(os.Stdout, f)
	l := newLogger(mw)
	return klog.NewHelper(l), func() error { return f.Close() }, nil
}
