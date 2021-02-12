/*
	golang local storage
	more info:
		https://github.com/qw4990/blog/blob/master/golang/gls/README.md
		https://github.com/jtolds/gls
		https://github.com/xiezhenye/gls
 */
package gls

import (
	"reflect"
	"runtime"
)

type flagFunc func(rem uint64, cb func())

var fs []flagFunc

const (
    bits = 4
    mask = 0xf // (1 << bits) -1
)

func initFlagFuncs() {
	fs = []flagFunc{
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 00
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 01
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 02
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 03
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 04
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 05
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 06
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 07
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 08
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 09
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 0a
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 0b
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 0c
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 0d
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 0e
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 0f
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 10
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 11
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 12
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 13
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 14
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 15
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 16
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 17
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 18
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 19
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 1a
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 1b
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 1c
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 1d
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 1e
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 1f
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 20
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 21
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 22
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 23
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 24
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 25
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 26
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 27
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 28
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 29
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 2a
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 2b
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 2c
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 2d
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 2e
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 2f
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 30
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 31
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 32
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 33
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 34
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 35
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 36
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 37
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 38
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 39
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 3a
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 3b
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 3c
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 3d
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 3e
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 3f
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 40
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 41
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 42
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 43
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 44
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 45
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 46
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 47
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 48
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 49
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 4a
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 4b
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 4c
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 4d
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 4e
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 4f
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 50
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 51
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 52
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 53
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 54
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 55
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 56
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 57
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 58
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 59
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 5a
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 5b
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 5c
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 5d
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 5e
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 5f
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 60
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 61
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 62
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 63
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 64
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 65
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 66
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 67
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 68
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 69
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 6a
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 6b
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 6c
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 6d
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 6e
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 6f
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 70
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 71
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 72
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 73
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 74
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 75
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 76
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 77
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 78
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 79
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 7a
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 7b
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 7c
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 7d
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 7e
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 7f
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 80
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 81
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 82
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 83
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 84
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 85
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 86
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 87
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 88
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 89
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 8a
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 8b
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 8c
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 8d
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 8e
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 8f
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 90
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 91
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 92
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 93
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 94
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 95
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 96
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 97
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 98
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 99
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 9a
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 9b
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 9c
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 9d
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 9e
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // 9f
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // a0
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // a1
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // a2
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // a3
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // a4
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // a5
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // a6
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // a7
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // a8
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // a9
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // aa
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // ab
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // ac
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // ad
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // ae
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // af
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // b0
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // b1
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // b2
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // b3
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // b4
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // b5
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // b6
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // b7
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // b8
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // b9
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // ba
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // bb
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // bc
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // bd
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // be
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // bf
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // c0
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // c1
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // c2
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // c3
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // c4
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // c5
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // c6
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // c7
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // c8
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // c9
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // ca
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // cb
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // cc
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // cd
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // ce
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // cf
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // d0
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // d1
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // d2
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // d3
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // d4
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // d5
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // d6
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // d7
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // d8
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // d9
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // da
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // db
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // dc
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // dd
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // de
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // df
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // e0
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // e1
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // e2
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // e3
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // e4
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // e5
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // e6
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // e7
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // e8
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // e9
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // ea
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // eb
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // ec
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // ed
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // ee
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // ef
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // f0
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // f1
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // f2
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // f3
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // f4
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // f5
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // f6
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // f7
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // f8
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // f9
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // fa
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // fb
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // fc
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // fd
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // fe
		func(rem uint64, cb func()) { if rem == 0 { cb() } else { fs[rem & mask](rem >> bits, cb) } }, // ff
	}
}

var startPc uintptr
var pcToN map[uintptr]uint64

func SetGID(gid uint64, cb func()) {
	if gid == 0 {
		cb()
	} else {
		fs[gid&mask](gid>>bits, cb)
	}
}

func init() {
	initFlagFuncs()
    pcToN = make(map[uintptr]uint64, len(fs))
	for i := 0; i < len(fs); i++ {
		pc := reflect.ValueOf(fs[i]).Pointer()
		pcToN[pc] = uint64(i)
	}
	startPc = reflect.ValueOf(SetGID).Pointer()
}

func GetGID() uint64 {
	var ret uint64 = 0
	for i := 1; ; i++ {
		pc, _, _, ok := runtime.Caller(i)
		if ! ok {
			break
		}
		fpc := runtime.FuncForPC(pc).Entry()
		n, ok := pcToN[fpc]
		if ok {
			ret <<= bits
			ret += n
		}
		if fpc == startPc {
			break
		}
	}
	return ret
}
