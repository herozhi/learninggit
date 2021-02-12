package runtime

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
)

func ForceGCHandler(w http.ResponseWriter, r *http.Request) {
	runtime.GC()
	w.WriteHeader(200)
	w.Write([]byte("forced gc"))
}

func ReadMemStatsHandler(w http.ResponseWriter, r *http.Request) {
	m := runtime.MemStats{}
	runtime.ReadMemStats(&m)
	b, err := json.Marshal(m)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("json marshal failed, err: "))
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(200)
	w.Write(b)
}

func GOMAXPROCSHandler(w http.ResponseWriter, r *http.Request) {
	n := runtime.GOMAXPROCS(0)
	w.WriteHeader(200)
	w.Write([]byte(strconv.Itoa(n)))
}

func NumCPUHandler(w http.ResponseWriter, r *http.Request) {
	n := runtime.NumCPU()
	w.WriteHeader(200)
	w.Write([]byte(strconv.Itoa(n)))
}

func VersionHandler(w http.ResponseWriter, r *http.Request) {
	s := runtime.Version()
	w.WriteHeader(200)
	w.Write([]byte(s))
}
