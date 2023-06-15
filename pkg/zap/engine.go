package zap

import (
	"io"
	"os"
)

var Log *Logger

func Step(path string) {
	if Log == nil {
		opts := []Option{
			// 附加日志调用信息
			WithCaller(true),
			AddCallerSkip(1),
		}
		Log = New(io.MultiWriter(os.Stdout, NewProductionRotateByTime(path)), DebugLevel, opts...)
	}
}
