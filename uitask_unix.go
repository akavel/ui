// +build !windows,!darwin,!plan9

// 16 february 2014

package ui

import (
	"fmt"
	"runtime"
)

// #cgo pkg-config: gtk+-3.0
// #include "gtk_unix.h"
import "C"

var uitask chan func()

func ui(main func()) error {
	runtime.LockOSThread()

	uitask = make(chan func())
	err := gtk_init()
	if err != nil {
		return fmt.Errorf("gtk_init() failed: %v", err)
	}

	// thanks to tristan and Daniel_S in irc.gimp.net/#gtk
	// see our_idle_callback in callbacks_unix.go for details
	go func() {
		for f := range uitask {
			done := make(chan struct{})
			gdk_threads_add_idle(&gtkIdleOp{
				what: f,
				done: done,
			})
			<-done
			close(done)
		}
	}()

	go func() {
		main()
		uitask <- gtk_main_quit
	}()

	C.gtk_main()
	return nil
}
