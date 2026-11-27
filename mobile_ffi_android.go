//go:build android && cgo

package main

/*
#include <stdlib.h>
*/
import "C"

import "unsafe"

//export WinminerCall
func WinminerCall(request *C.char) *C.char {
	if request == nil {
		return C.CString(`{"error":"missing request"}`)
	}
	return C.CString(callMobileAPI(C.GoString(request)))
}

//export WinminerFree
func WinminerFree(value *C.char) {
	C.free(unsafe.Pointer(value))
}
