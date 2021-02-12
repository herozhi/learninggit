package logs

import (
	"net/http"
	"strconv"

	"code.byted.org/gopkg/logs"
)

func SetLevelHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("parse form failed: "))
		w.Write([]byte(err.Error()))
		return
	}
	levelStr := r.PostForm.Get("level")
	if levelStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("field \"level\" is required."))
		return
	}
	level, err := strconv.ParseInt(levelStr, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid level "))
		w.Write([]byte(levelStr))
		return
	}

	logs.SetLevel(int(level))
	w.WriteHeader(200)
	w.Write([]byte("set log level succeeded"))
}
