package src

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

type AvahiService struct {
	Interface   string // e.g. "eno1"
	Protocol    string // e.g. "IPv4"
	Name        string
	ServiceType string
	Domain      string
	Hostname    string
	Address     string
	Port        string
	TxtRecords  []string
}

func getAvahiServices() <-chan []AvahiService {
	res := make(chan []AvahiService)
	go func() {
		defer close(res)

		cmd := exec.Command("avahi-browse", "-arpt")
		out, err := cmd.Output()
		if err != nil {
			log.Printf("ERROR: Failed to get Avahi services: %v", err)
			res <- nil
			return
		}

		lines := strings.Split(string(out), "\n")
		var services []AvahiService
		for _, line := range lines {
			if line == "" {
				continue
			}

			parts := strings.Split(line, ";")
			switch parts[0] {
			case "+":
				services = append(services, AvahiService{
					Interface:   parts[1],
					Protocol:    parts[2],
					Name:        parts[3],
					ServiceType: parts[4],
					Domain:      parts[5],
				})
			case "-":
				// do nothing; we don't care about entries getting removed for now
			case "=":
				var service *AvahiService
				for i := range services {
					svc := &services[i]
					if svc.Interface == parts[1] && svc.Protocol == parts[2] && svc.Name == parts[3] && svc.ServiceType == parts[4] && svc.Domain == parts[5] {
						service = svc
						break
					}
				}
				if service == nil {
					log.Printf("WARNING: Couldn't find service %s %s %s %s %s in list", parts[1], parts[2], parts[3], parts[4], parts[5])
					break
				}

				service.Hostname = parts[6]
				service.Address = parts[7]
				service.Port = parts[8]
				service.TxtRecords = []string{parts[9]} // TODO: Actually split
			default:
				panic(fmt.Errorf("unknown line type %v", parts[0]))
			}
		}

		res <- services
	}()

	return res
}

type AvahiServiceType struct {
	DNSSDName string
	NiceName  string
}

func getAvahiServiceTypes() []AvahiServiceType {
	var raw, nice []byte
	{
		cmd := exec.Command("avahi-browse", "--dump-db")
		var err error
		nice, err = cmd.Output()
		if err != nil {
			panic(err)
		}
	}
	{
		cmd := exec.Command("avahi-browse", "--dump-db", "--no-db-lookup")
		var err error
		raw, err = cmd.Output()
		if err != nil {
			panic(err)
		}
	}

	var res []AvahiServiceType
	rawParts := strings.Split(string(raw), "\n")
	niceParts := strings.Split(string(nice), "\n")
	for i := range rawParts {
		rawPart := rawParts[i]
		nicePart := niceParts[i]
		if rawPart == "" {
			continue
		}
		res = append(res, AvahiServiceType{
			DNSSDName: rawPart,
			NiceName:  nicePart,
		})
	}
	return res
}
