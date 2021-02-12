package compress

import (
	"compress/gzip"
	"github.com/gin-gonic/gin"
)

type gzipWriter struct {
	gin.ResponseWriter
	writer *gzip.Writer
}

func NewGzipWriter(w gin.ResponseWriter) *gzipWriter {
	wb,_ := gzip.NewWriterLevel(w, 6)
	return &gzipWriter{w, wb}
}

func (w *gzipWriter) Write(data [] byte) (int, error) {
	return w.writer.Write(data)
}

func (w *gzipWriter) WriteHeader(code int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipWriter) WriteString(s string) (int, error) {
	return w.writer.Write([] byte (s))
}