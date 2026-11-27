//go:build darwin && cgo
// +build darwin,cgo

package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"unsafe"

	"github.com/BurntSushi/toml"
)

//go:embed winminer.toml
var mobileDefaultConfig []byte

var mobileRuntime struct {
	once   sync.Once
	server *webServer
	err    error
}

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

func callMobileAPI(raw string) string {
	var request mobileRequest
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return mobileError(err)
	}
	if err := decoder.Decode(new(interface{})); err != io.EOF {
		return mobileError(errors.New("request must contain one JSON value"))
	}

	mobileRuntime.once.Do(func() {
		if _, err := toml.Decode(string(mobileDefaultConfig), &config); err != nil {
			mobileRuntime.err = fmt.Errorf("parse embedded configuration: %w", err)
			return
		}
		if len(config.Levels) == 0 {
			mobileRuntime.err = errors.New("configuration has no levels")
			return
		}
		mobileRuntime.server = newMobileServer()
	})
	if mobileRuntime.err != nil {
		return mobileError(mobileRuntime.err)
	}

	result, err := dispatchMobileRequest(mobileRuntime.server, request)
	if err != nil {
		return mobileError(err)
	}
	var response bytes.Buffer
	if err := json.NewEncoder(&response).Encode(result); err != nil {
		return mobileError(err)
	}
	return strings.TrimSpace(response.String())
}

func mobileError(err error) string {
	response, marshalErr := json.Marshal(map[string]string{"error": err.Error()})
	if marshalErr != nil {
		return `{"error":"failed to encode error"}`
	}
	return string(response)
}
