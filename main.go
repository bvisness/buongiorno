package main

import (
	"fmt"
	"runtime"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/backend/sdlbackend"
	"github.com/AllenDang/cimgui-go/imgui"
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

	currentBackend.Run(Loop)
}

var (
	showDemoWindow bool
	value3         int32
	values         [2]int32
	content        string = "Let me try"
	color4         [4]float32
	selected       bool
)

func Loop() {
	imgui.ClearSizeCallbackPool()
	ShowWidgetsDemo()
}

func ShowWidgetsDemo() {
	if showDemoWindow {
		imgui.ShowDemoWindowV(&showDemoWindow)
	}

	imgui.SetNextWindowSizeV(imgui.NewVec2(300, 300), imgui.CondOnce)
	imgui.SetNextWindowSizeConstraintsV(imgui.NewVec2(300, 300), imgui.NewVec2(500, 500), func(data *imgui.SizeCallbackData) {
	}, 0)

	imgui.Begin("Window 1")
	if imgui.ButtonV("Click Me", imgui.NewVec2(80, 20)) {
		fmt.Println("Button clicked")
	}
	imgui.TextUnformatted("Unformatted text")
	imgui.Checkbox("Show demo window", &showDemoWindow)
	if imgui.BeginCombo("Combo", "Combo preview") {
		imgui.SelectableBoolPtr("Item 1", &selected)
		imgui.SelectableBool("Item 2")
		imgui.SelectableBool("Item 3")
		imgui.EndCombo()
	}

	if imgui.RadioButtonBool("Radio button1", selected) {
		selected = true
	}

	imgui.SameLine()

	if imgui.RadioButtonBool("Radio button2", !selected) {
		selected = false
	}

	imgui.InputTextWithHint("Name", "write your name here", &content, 0, InputTextCallback)
	imgui.Text(content)
	imgui.SliderInt("Slider int", &value3, 0, 100)
	imgui.DragInt("Drag int", &values[0])
	imgui.DragInt2("Drag int2", &values)
	imgui.ColorEdit4("Color Edit3", &color4)
	imgui.End()
}

func InputTextCallback(data imgui.InputTextCallbackData) int {
	fmt.Println("got call back")
	return 0
}
