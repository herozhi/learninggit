package debug

import (
	"encoding/json"
	"net/http"
	"runtime/debug"
	"strconv"
)

func FreeOSMemoryHandler(w http.ResponseWriter, r *http.Request) {
	debug.FreeOSMemory()
	w.WriteHeader(200)
	w.Write([]byte("os mem freed"))
}

func BuildInfoHandler(w http.ResponseWriter, r *http.Request) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		w.WriteHeader(http.StatusNotImplemented)
		w.Write([]byte("The information is available only in binaries built with module support."))
		return
	}

	b, err := json.Marshal(info)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("json marshal failed, err: "))
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(200)
	w.Write(b)
}

func SetGCPercentHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("parse form failed: "))
		w.Write([]byte(err.Error()))
		return
	}
	percentStr := r.PostForm.Get("percent")
	if percentStr == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("field \"percent\" is required."))
		return
	}
	percent, err := strconv.ParseInt(percentStr, 10, 64)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid percent "))
		w.Write([]byte(percentStr))
		return
	}

	ret := debug.SetGCPercent(int(percent))
	w.WriteHeader(200)
	w.Write([]byte("previous gc setting is: "))
	w.Write([]byte(strconv.FormatInt(int64(ret), 10)))
}

func ReadGCStatsHandler(w http.ResponseWriter, r *http.Request) {
	stats := debug.GCStats{}
	debug.ReadGCStats(&stats)
	b, err := json.Marshal(stats)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("json marshal failed, err: "))
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(200)
	w.Write(b)
}
