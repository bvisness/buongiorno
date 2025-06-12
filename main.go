package main

import (
	"fmt"
	"runtime"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/backend/sdlbackend"
	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/bvisness/buongiorno/src"
)

func init() {
	runtime.LockOSThread()
}

func main() {
	currentBackend, err := backend.CreateBackend(sdlbackend.NewSDLBackend())
	if err != nil {
		panic(err)
	}

	currentBackend.SetBgColor(imgui.NewVec4(0.1, 0.1, 0.1, 1.0))
	currentBackend.CreateWindow("Buongiorno", 1200, 900)

	currentBackend.SetDropCallback(func(p []string) {
		fmt.Printf("drop triggered: %v", p)
	})
	currentBackend.SetCloseCallback(func() {
		fmt.Println("window is closing")
	})

	currentBackend.Run(func() {
		imgui.ClearSizeCallbackPool()
		src.UI()
	})
}
