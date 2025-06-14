package src

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	_ "image/png"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/bvisness/buongiorno/src/packet"
	"github.com/bvisness/buongiorno/src/utils"
	"github.com/miekg/dns"
)

var (
	avahiServices []AvahiService
	serviceTypes  = utils.GroupIntoMap(getAvahiServiceTypes(), func(t AvahiServiceType) string {
		return t.DNSSDName
	})

	//go:embed macbook-line.png
	macbookRaw []byte
	macbook    *backend.Texture
)

type ServiceInstance struct {
	InstanceName string
	ServiceType  string // the raw DNS-SD service type, e.g. _airplay._tcp
	Domain       string // should always be "local" for this project

	Host string // Optional. Will be filled in by a corresponding SRV record.
	Port int    // Optional. Will be filled in by a corresponding SRV record.

	Extras []string // May be filled in by a corresponding TXT record.

	RawName string // the raw Service Instance Name from the PTR record
}

type Host struct {
	Name     string
	IPv4Addr string
	IPv6Addr string
}

var (
	serviceInstances []ServiceInstance
	hosts            []Host

	// SRV and TXT records are deferred to the end of packet processing to ensure
	// that we always process their info after any PTRs.
	deferredRRs []dns.RR
)

func init() {
	go func() {
		t := utils.NewInstaTicker(time.Second * 1)
		for range t.C {
			svcs := getAvahiServices()
			avahiServices = <-svcs
		}
	}()

	packets, err := packet.CaptureMDNS()
	if err != nil {
		panic(fmt.Errorf("Could not start packet capture: %v", err))
	}
	go func() {
		for p := range packets {
			// DNS-SD recommends that various records be added to the Additional
			// section in order to flesh out the services being advertised. This
			// effectively means that we can just treat whatever we find in the
			// Additional section as if they were extra answers. (Also, it seems like
			// sometimes we get them in the Authoritative Nameservers section too?
			// Who knows, just watch 'em all.)
			//
			// https://datatracker.ietf.org/doc/html/rfc6763#section-12
			//
			// One quirk worth being aware of is that mDNS queries can contain known
			// answers in the Answers section. For this project we naively trust them
			// because it fleshes out our graph and because any incorrect entries
			// will be overridden by a subsequent answer anyway. (Unless they were
			// unicasted back I suppose, but there's only so much I can do, all
			// right?)
			rrs := p.DNS.Answer
			rrs = append(rrs, p.DNS.Extra...)
			rrs = append(rrs, p.DNS.Ns...)
			for _, answer := range rrs {
				// In DNS-SD, a PTR record indicates that a service is being
				// advertised. If a PTR record is provided than it is expected that
				// a SRV and TXT record will also be provided (although this is
				// seemingly not guaranteed, from my testing).
				//
				// The PTR record itself simply contains a Service Instance Name
				// (https://datatracker.ietf.org/doc/html/rfc6763#section-4.1). For
				// example, a PTR record for name "_airplay._tcp.local" may map to
				// the service instance "MacBook Pro (3)._airplay._tcp.local", where
				// "MacBook Pro (3)" is the instance, "_airplay._tcp" is the service,
				// and "local" is the domain.
				//
				// The corresponding SRV and TXT records would have the name
				// "MacBook Pro (3)._airplay._tcp.local". The SRV record tells the
				// mDNS client what host and port to use for the advertised instance,
				// for example, "MacBook-Pro-3.local" and port 7000. The TXT record
				// would provide any additional data about the service, e.g. AirPlay
				// protocol version.
				//
				// https://datatracker.ietf.org/doc/html/rfc6763#section-5
				//
				// A PTR record for "_services._dns-sd._udp.local" is used for
				// enumeration of all available services. These are PTRs to PTRS, and
				// will not have corresponding SRV and TXT records. For this tool we
				// will actually just ignore those since they contain no data to
				// identify a specific instance or host, so they're pretty irrelevant.
				//
				// https://datatracker.ietf.org/doc/html/rfc6763#section-9

				switch rr := answer.(type) {
				case *dns.PTR:
					log.Printf("Got PTR: %#v", rr)
					if packet.HostMatches(rr.Hdr.Name, "_services._dns-sd._udp.local") {
						// Meta-PTR. Ignore, since they contain nothing to identify a
						// specific instance. We will see the PTRs we care about in other
						// records.
						break
					}

					if !packet.HostMatches(rr.Hdr.Name, "**._tcp.local") && !packet.HostMatches(rr.Hdr.Name, "**._udp.local") {
						// This PTR is not advertising a service instance.
						break
					}

					serviceInstanceName := rr.Ptr
					snameParts := packet.SplitHost(serviceInstanceName)
					instance := ServiceInstance{
						InstanceName: snameParts[0],
						ServiceType:  strings.Join(snameParts[1:len(snameParts)-1], "."),
						Domain:       snameParts[len(snameParts)-1], // assuming "local" always per above

						RawName: serviceInstanceName,
					}
					utils.AppendToSliceIfAbsent(&serviceInstances, instance, func(i ServiceInstance) string {
						return i.RawName
					})

				// SRV and TXT records go into the queue.
				case *dns.SRV:
					log.Printf("Got SRV: %#v", rr)
					if !packet.HostMatches(rr.Hdr.Name, "**._tcp.local") && !packet.HostMatches(rr.Hdr.Name, "**._udp.local") {
						// This SRV has nothing to do with a service instance.
						break
					}
					utils.AppendToSliceIfAbsent[dns.RR, string](&deferredRRs, rr, func(r dns.RR) string { return r.Header().Name })
				case *dns.TXT:
					log.Printf("Got TXT: %#v", rr)
					if !packet.HostMatches(rr.Hdr.Name, "**._tcp.local") && !packet.HostMatches(rr.Hdr.Name, "**._udp.local") {
						// This TXT has nothing to do with a service instance.
						break
					}
					utils.AppendToSliceIfAbsent[dns.RR, string](&deferredRRs, rr, func(r dns.RR) string { return r.Header().Name })

				// A and AAAA records get tracked to their corresponding hosts.
				case *dns.A:
					log.Printf("Got A: %#v", rr)
					host := utils.AppendToSliceIfAbsent(&hosts, Host{Name: rr.Hdr.Name}, func(h Host) string { return h.Name })
					host.IPv4Addr = rr.A.String()
				case *dns.AAAA:
					log.Printf("Got AAAA: %#v", rr)
					host := utils.AppendToSliceIfAbsent(&hosts, Host{Name: rr.Hdr.Name}, func(h Host) string { return h.Name })
					host.IPv6Addr = rr.AAAA.String()
				}
			}

			// Process the queue of SRVs and TXTs. Because we process items in order,
			// even a stack of old records should resolve quickly to the latest
			// information.
			for i := 0; i < len(deferredRRs); i++ {
				rr := deferredRRs[i]
				if instance, ok := utils.FindInSlice(serviceInstances, func(i ServiceInstance) bool {
					return i.RawName == rr.Header().Name
				}); ok {
					// We have an instance we can update.
					switch rr := rr.(type) {
					case *dns.SRV:
						instance.Host = rr.Target
						instance.Port = int(rr.Port)
					case *dns.TXT:
						instance.Extras = rr.Txt
					}

					// Since we processed this record, remove it from the queue.
					deferredRRs = slices.Delete(deferredRRs, i, i+1)
					i -= 1
				} else {
					// Still no information for this record.
				}
			}
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

		for _, service := range avahiServices {
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

		grouped := utils.GroupIntoSlice(avahiServices, func(s AvahiService) string { return s.Hostname })
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

	if imgui.Begin("Debug") {
		imgui.Text("Services:")
		for _, instance := range serviceInstances {
			if imgui.TreeNodeExStr(instance.RawName) {
				imgui.Text(fmt.Sprintf("Name: %s", instance.InstanceName))
				imgui.Text(fmt.Sprintf("ServiceType: %s", instance.ServiceType))
				imgui.Text(fmt.Sprintf("Domain: %s", instance.Domain))
				imgui.Text(fmt.Sprintf("Host: %s", instance.Host))
				imgui.Text(fmt.Sprintf("Port: %d", instance.Port))
				if imgui.TreeNodeExStrStr("extras", 0, "Extras") {
					for _, extra := range instance.Extras {
						imgui.Text(extra)
					}
					imgui.TreePop()
				}

				imgui.TreePop()
			}
		}

		imgui.Text("Hosts:")
		for _, host := range hosts {
			if imgui.TreeNodeExStr(host.Name) {
				imgui.Text(fmt.Sprintf("Name: %s", host.Name))
				imgui.Text(fmt.Sprintf("IPv4 Addr: %s", host.IPv4Addr))
				imgui.Text(fmt.Sprintf("IPv6 Addr: %s", host.IPv6Addr))

				imgui.TreePop()
			}
		}

		imgui.Text(fmt.Sprintf("%d queued RRs:", len(deferredRRs)))
		for _, rr := range deferredRRs {
			imgui.BulletText(fmt.Sprintf("%s %s", dns.Type(rr.Header().Rrtype), rr.Header().Name))
		}
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
