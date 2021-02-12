package stress

import (
	"fmt"

	"code.byted.org/gin/ginex/configstorer"
	"code.byted.org/gin/ginex/internal"
	"code.byted.org/gopkg/logs"
	"github.com/gin-gonic/gin"
)

func StressSwitcher(psm string, cluster string) gin.HandlerFunc {
	psmKey := fmt.Sprintf("/kite/stressbot/%s/%s/request/switch", psm, cluster)
	globalKey := "/kite/stressbot/request/switch/global"
	configstorer.Get(globalKey)
	configstorer.Get(psmKey)

	return func(ctx *gin.Context) {
		stressTag := ctx.Request.Header.Get(internal.TT_STRESS_KEY)

		toReject := false
		reason := ""

		defer func() {
			if r := recover(); r != nil {
				logs.Errorf("Stress middleware panic: %v", r)
				ctx.Next()
			} else {
				if toReject {
					ctx.String(403, fmt.Sprintf("reject stress bot request: %s", reason))
					ctx.Abort()
				} else {
					ctx.Next()
				}
			}
		}()

		if stressTag != "" {
			toReject = true
			globalSwitch, gerr := configstorer.GetOrDefault(globalKey, "off")
			if gerr != nil {
				toReject = true
				reason = "get config error"
				return
			} else if globalSwitch == "off" {
				reason = "global switch off"
				toReject = true
				return
			} else if globalSwitch != "on" {
				toReject = true
				reason = "error switch value"
			}

			psmClusterSwitch, perr := configstorer.GetOrDefault(psmKey, "off")
			if perr != nil {
				reason = "get config error"
				toReject = true
				return
			} else if psmClusterSwitch == "off" {
				reason = fmt.Sprintf("psm:cluster %s:%s switch off", psm, cluster)
				toReject = true
				return
			} else if psmClusterSwitch == "on" {
				toReject = false
			} else {
				toReject = true
				reason = "error switch value"
			}
		}
	}
}
