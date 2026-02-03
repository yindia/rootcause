//go:build windows

package server

import "os"

func notifyReload(ch chan<- os.Signal) {
	_ = ch
}
