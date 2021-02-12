package goredis

import (
	dps "code.byted.org/inf/infsecc"
	"errors"
)

func GetRedisDpsToken(forceupdate bool) string {
	//FIXME: check the atomicity of GetToken
	dpsToken, err := dps.GetToken(forceupdate)
	if err != nil {
		return "0"
	} else {
		return "1" + dpsToken
	}
}

func VerifyRedisDpsReply(resp string) error {
	if resp == "OK" {
		return nil
	} else {
		return errors.New(resp)
	}
}
