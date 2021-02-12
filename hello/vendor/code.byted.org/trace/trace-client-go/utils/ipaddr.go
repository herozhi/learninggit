package utils

import (
	"fmt"
	"math/big"
	"net"
)

func InetAtoN(ipstr string) uint32 {
	ret := big.NewInt(0)
	ret.SetBytes(net.ParseIP(ipstr).To4())
	return uint32(ret.Uint64())
}

func InetNtoA(ipval uint32) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		byte(ipval>>24), byte(ipval>>16), byte(ipval>>8), byte(ipval))
}
