package render

import (
	"fmt"
	"time"
	"errors"
	"html/template"
	"github.com/gin-gonic/gin/render"
	"code.byted.org/gopkg/logs"
)

type AutoReloadRender struct {
	Render *render.HTMLDebug
	Dur    time.Duration

	ts  time.Time
	tpl *template.Template
}

func (r *AutoReloadRender) reload(name string, data interface{}) (result render.HTML, err error) {
	defer func() {
		if e := recover(); e != nil {
			err = errors.New(fmt.Sprintf("detail error, %v", e))
		}
	}()
	result = r.Render.Instance(name, data).(render.HTML)
	return
}

func (r *AutoReloadRender) Instance(name string, data interface{}) render.Render {

	n := time.Now()
	if nil == r.tpl || n.UTC().Sub(r.ts.UTC()) > r.Dur {

		r.ts = n
		if htm, err := r.reload(name, data); nil == err {
			r.tpl = htm.Template
		} else {
			logs.Warnf("Reloading templates failure: %v", err)
		}
	}

	return render.HTML{
		Template: r.tpl,
		Name:     name,
		Data:     data,
	}
}
