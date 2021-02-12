package compress

import (
	"fmt"
	"strings"

	"code.byted.org/gin/ginex/configstorer"
	"github.com/gin-gonic/gin"
)

func compressType(c *gin.Context, supported string) string {
	compressed := c.GetHeader("Accept-Encoding")
	compressTypes := strings.Split(strings.Replace(strings.ToLower(compressed), " ", "", -1), ",")
	compressSet := map[string]int{}

	supportedTypes := strings.Split(strings.Replace(strings.ToLower(supported), " ", "", -1), ",")
	supportedSet := map[string]int{}

	for _, supportedType := range supportedTypes {
		supportedSet[supportedType] = 1
	}

	for _, compressType := range compressTypes {
		if _, b := supportedSet[compressType]; b {
			compressSet[compressType] = 1
		}
	}

	if _, b := compressSet["br"]; b {
		return "br"
	}

	if _, b := compressSet["gzip"]; b {
		return "gzip"
	}

	return ""
}

func Compress(psm string) gin.HandlerFunc {
	psmKey := fmt.Sprintf("/kite/compress/%s/request/supported", psm)

	return func(c *gin.Context) {
		supported, err := configstorer.GetOrDefault(psmKey, "")
		if err != nil {
			return
		}

		compressType := compressType(c, supported)

		var w interface{}

		switch compressType {
		case "gzip":
			w = NewGzipWriter(c.Writer)
			c.Writer = w.(*gzipWriter)
		case "br":
			w = NewBrotliWriter(c.Writer)
			c.Writer = w.(*brotliWriter)
		default:
			return
		}

		c.Header("Content-Encoding", compressType)
		c.Header("Vary", "Accept-Encoding")

		defer func() {
			switch compressType {
			case "gzip":
				w.(*gzipWriter).writer.Flush()
				w.(*gzipWriter).writer.Close()
			case "br":
				w.(*brotliWriter).writer.Flush()
				w.(*brotliWriter).writer.Close()
			default:
				return
			}
		}()

		c.Next()
	}
}
