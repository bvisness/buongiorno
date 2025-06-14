package src

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	_ "image/png"
	"log"
	"math"
	"math/rand"
	"net"
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

	lastFrame time.Time
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

	// Node graph stuff
	Pos, Vel imgui.Vec2
}

type ServiceQuery struct {
	SourceAddr  string
	ServiceType string // the raw DNS-SD service type, e.g. _airplay._tcp

	RawQuery string
}

var (
	serviceInstances []ServiceInstance
	hosts            []Host
	serviceQueries   []ServiceQuery

	// SRV and TXT records are deferred to the end of packet processing to ensure
	// that we always process their info after any PTRs.
	deferredRRs []dns.RR
)

func init() {
	// hosts = append(hosts,
	// 	Host{Name: "a"},
	// 	Host{Name: "b"},
	// 	Host{Name: "c"},
	// )

	me := Host{Name: "This PC"}
	if en0, err := net.InterfaceByName("en0"); err != nil {
		if addrs, err := en0.Addrs(); err != nil {
			for _, addr := range addrs {
				var ip net.IP
				switch addr := addr.(type) {
				case *net.IPNet:
					ip = addr.IP
				case *net.IPAddr:
					ip = addr.IP
				}

				if ip.To4() == nil {
					me.IPv6Addr = ip.String()
				} else {
					me.IPv4Addr = ip.String()
				}
			}
		} else {
			log.Print("No addrs found for interface en0")
		}
	} else {
		log.Print("No interface found named en0")
	}
	hosts = append(hosts, me)

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
			// Track queries for PTR records
			for _, question := range p.DNS.Question {
				switch question.Qtype {
				case dns.TypePTR:
					if !packet.HostMatches(question.Name, "**._tcp.local") && !packet.HostMatches(question.Name, "**._udp.local") {
						// This PTR question is not looking for DNS-SD services
						break
					}

					nameParts := packet.SplitHost(question.Name)
					serviceQueries = append(serviceQueries, ServiceQuery{
						SourceAddr:  p.SrcAddr,
						ServiceType: strings.Join(nameParts[:len(nameParts)-1], "."),

						RawQuery: question.Name,
					})
				}
			}

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
			answers := p.DNS.Answer
			answers = append(answers, p.DNS.Extra...)
			answers = append(answers, p.DNS.Ns...)
			for _, answer := range answers {
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

func serviceQueriesForHost(host Host) []ServiceQuery {
	var res []ServiceQuery
	seen := make(map[string]struct{})
	for _, query := range serviceQueries {
		if query.SourceAddr != host.IPv4Addr && query.SourceAddr != host.IPv6Addr {
			continue
		}
		if _, alreadySeen := seen[query.RawQuery]; alreadySeen {
			continue
		}

		res = append(res, query)
		seen[query.RawQuery] = struct{}{}
	}
	return res
}

func AfterCreateContext() {
	macbook = loadTexture(macbookRaw)
	lastFrame = time.Now()
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
				imgui.Text(fmt.Sprintf("Position: [%f, %f]", host.Pos.X, host.Pos.Y))

				queries := serviceQueriesForHost(host)
				if len(queries) > 0 {
					imgui.Text("Requested services:")
					imgui.Indent()
					for _, query := range queries {
						if niceName, ok := serviceTypes[query.ServiceType]; ok {
							imgui.Text(fmt.Sprintf("%s (%s)", niceName[0].NiceName, query.ServiceType))
						} else {
							imgui.Text(query.ServiceType)
						}
					}
					imgui.Unindent()
				}

				imgui.TreePop()
			}
		}

		imgui.Text(fmt.Sprintf("%d queued RRs:", len(deferredRRs)))
		for _, rr := range deferredRRs {
			imgui.BulletText(fmt.Sprintf("%s %s", dns.Type(rr.Header().Rrtype), rr.Header().Name))
		}
	}
	imgui.End()

	if imgui.Begin("Graph Controls") {
		imgui.DragFloat("Spring Length", &springLength)
		imgui.DragFloat("Spring Strength", &springStrength)
		imgui.DragFloat("Repel Strength", &repelStrength)
		imgui.SliderFloatV("Gravity Strength", &gravityStrength, 0, 1, "%.3f", 0)
		imgui.SliderFloatV("Damping", &damping, 0, 0.1, "%.3f", 0)
	}
	imgui.End()

	if imgui.Begin("Devices") {
		// Update graph
		now := time.Now()
		dt := now.Sub(lastFrame)
		if dt > 100*time.Millisecond {
			dt = 100 * time.Millisecond
		}
		updateGraph(float32(dt.Seconds()))
		lastFrame = now

		windowPos := imgui.CursorScreenPos()
		windowCenter := windowPos.Add(imgui.NewVec2(imgui.WindowWidth()/2, imgui.WindowHeight()/2))

		// Render graph lines
		// dl := imgui.WindowDrawList()
		// dl.AddLine(imgui.NewVec2(50, 50).Add(windowPos), imgui.NewVec2(150, 100).Add(windowPos), imgui.ColorU32Vec4(imgui.NewVec4(1, 1, 1, 1)))

		// Render graph nodes
		for _, host := range hosts {
			imgui.SetCursorScreenPos(windowCenter.Add(host.Pos))
			imgui.BeginChildStr(host.Name)
			{
				imgui.Image(macbook.ID, imgui.NewVec2(22, 18))
				imgui.Text(host.Name)
				if host.IPv4Addr != "" {
					imgui.Text(host.IPv4Addr)
				}
				if host.IPv6Addr != "" {
					imgui.Text(host.IPv6Addr)
				}
			}
			imgui.EndChild()
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

var (
	springLength    float32 = 100
	springStrength  float32 = 0.01
	repelStrength   float32 = 25000
	gravityStrength float32 = 0.01 // gravity is like a spring to the center
	damping         float32 = 0.02
	maxVelocity     float32 = 25
	newNodeJitter   float32 = 20
)

func updateGraph(dt float32) {
	for i := range hosts {
		host := &hosts[i]

		// Jitter to ensure force directed stuff has something to work with
		if host.Pos.X == 0 && host.Pos.Y == 0 {
			host.Pos = imgui.NewVec2(
				newNodeJitter*2*rand.Float32()-newNodeJitter, newNodeJitter*2*rand.Float32()-newNodeJitter,
			)
		}

		// Gravity
		toOrigin := host.Pos.Mul(-1)
		host.Vel.X += gravityStrength * toOrigin.X
		host.Vel.Y += gravityStrength * toOrigin.Y
	}

	// Repulsion
	for i := range hosts {
		for j := range hosts {
			a, b := &hosts[i], &hosts[j]
			dx := a.Pos.X - b.Pos.X
			dy := a.Pos.Y - b.Pos.Y
			dist2 := dx*dx + dy*dy
			force := repelStrength / (dist2 + 0.01) // force falls off with the square of the distance
			d := float32(math.Sqrt(float64(dist2)))
			fx := force * (dx / (d + 0.01))
			fy := force * (dy / (d + 0.01))
			a.Vel.X += fx
			a.Vel.Y += fy
		}
	}

	// Integrate
	for i := range hosts {
		host := &hosts[i]

		// Update velocity (damping + clamping)
		host.Vel = host.Vel.Mul(1 - damping)
		host.Vel.X = utils.Clamp(host.Vel.X, -maxVelocity, maxVelocity)

		host.Pos.X += host.Vel.X * dt
		host.Pos.Y += host.Vel.Y * dt
	}
}
