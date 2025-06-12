package src

import (
	"fmt"

	"github.com/AllenDang/cimgui-go/imgui"
)

var (
	showDemoWindow bool
	value3         int32
	values         [2]int32
	content        string = "Let me try"
	color4         [4]float32
	selected       bool
)

func UI() {
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
