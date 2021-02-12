package compress

import (
	"github.com/andybalholm/brotli"
	"github.com/gin-gonic/gin"
)

type brotliWriter struct {
	gin.ResponseWriter
	writer *brotli.Writer
}


func NewBrotliWriter(w gin.ResponseWriter) *brotliWriter {
	wb := brotli.NewWriterLevel(w, 5)
	return &brotliWriter{w, wb}
}

func (w *brotliWriter) Write(data [] byte ) (int, error) {
	return w.writer.Write(data)
}

func (w *brotliWriter) WriteString(s string) (int, error) {
	return w.writer.Write([] byte(s))
}

func (w *brotliWriter) WriteHeader(code int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(code)
}
