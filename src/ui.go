package src

import (
	"fmt"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/bvisness/buongiorno/src/utils"
)

var (
	services []AvahiService
)

func init() {
	go func() {
		t := utils.NewInstaTicker(time.Second * 1)
		for range t.C {
			svcs := getAvahiServices()
			services = <-svcs
		}
	}()
}

func UI() {
	imgui.ShowDemoWindow()

	imgui.SetNextWindowSizeV(imgui.NewVec2(300, 300), imgui.CondOnce)

	imgui.Begin("Services")
	{
		// imgui.BeginTable("services", 8)
		imgui.BeginTableV("services", 8, imgui.TableFlagsSizingFixedFit|imgui.TableFlagsResizable|imgui.TableFlagsBorders|imgui.TableFlagsRowBg, imgui.NewVec2(0, 0), 0)

		imgui.TableSetupColumn("Interface")
		imgui.TableSetupColumn("Protocol")
		imgui.TableSetupColumn("Name")
		imgui.TableSetupColumn("Service Type")
		imgui.TableSetupColumn("Domain")
		imgui.TableSetupColumn("Hostname")
		imgui.TableSetupColumn("Address")
		imgui.TableSetupColumn("Port")
		imgui.TableHeadersRow()

		for _, service := range services {
			imgui.TableNextRow()
			imgui.TableNextColumn()
			imgui.Text(service.Interface)
			imgui.TableNextColumn()
			imgui.Text(service.Protocol)
			imgui.TableNextColumn()
			imgui.Text(service.Name)
			imgui.TableNextColumn()
			imgui.Text(service.ServiceType)
			imgui.TableNextColumn()
			imgui.Text(service.Domain)
			imgui.TableNextColumn()
			imgui.Text(service.Hostname)
			imgui.TableNextColumn()
			imgui.Text(service.Address)
			imgui.TableNextColumn()
			imgui.Text(service.Port)
		}
		imgui.EndTable()
	}
	imgui.End()
}

func InputTextCallback(data imgui.InputTextCallbackData) int {
	fmt.Println("got callback")
	return 0
}
