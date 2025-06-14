package src

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	_ "image/png"
	"time"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/bvisness/buongiorno/src/utils"
)

var (
	services     []AvahiService
	serviceTypes = utils.GroupIntoMap(getAvahiServiceTypes(), func(t AvahiServiceType) string {
		return t.DNSSDName
	})

	//go:embed macbook-line.png
	macbookRaw []byte
	macbook    *backend.Texture
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

func AfterCreateContext() {
	macbook = loadTexture(macbookRaw)
}

func UI() {
	imgui.ShowDemoWindow()

	imgui.SetNextWindowSizeV(imgui.NewVec2(300, 300), imgui.CondOnce)

	if imgui.Begin("Services") {
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

		grouped := utils.GroupIntoSlice(services, func(s AvahiService) string { return s.Hostname })
		for _, group := range grouped {
			if imgui.TreeNodeExStr(group.Key) {
				imgui.BeginTableV("services", 7, imgui.TableFlagsSizingFixedFit|imgui.TableFlagsBorders|imgui.TableFlagsRowBg, imgui.NewVec2(0, 0), 0)

				imgui.TableSetupColumn("Interface")
				imgui.TableSetupColumn("Protocol")
				imgui.TableSetupColumn("Name")
				imgui.TableSetupColumn("Service Type")
				imgui.TableSetupColumn("Domain")
				imgui.TableSetupColumn("Address")
				imgui.TableSetupColumn("Port")
				imgui.TableHeadersRow()

				for _, service := range group.Items {
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
					imgui.Text(service.Address)
					imgui.TableNextColumn()
					imgui.Text(service.Port)
				}
				imgui.EndTable()

				imgui.TreePop()
			}
		}

		imgui.Image(macbook.ID, imgui.NewVec2(22, 18))
	}
	imgui.End()
}

func InputTextCallback(data imgui.InputTextCallbackData) int {
	fmt.Println("got callback")
	return 0
}

func loadTexture(imgData []byte) *backend.Texture {
	img, _, err := image.Decode(bytes.NewReader(imgData))
	if err != nil {
		panic(err)
	}
	return backend.NewTextureFromRgba(backend.ImageToRgba(img))
}
