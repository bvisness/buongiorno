package packet

import (
	"fmt"
	"strings"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/miekg/dns"
)

func ParsePacket(packet []byte) (dns.Msg, error) {
	var msg dns.Msg
	if err := msg.Unpack(packet); err != nil {
		return dns.Msg{}, err
	}
	return msg, nil
}

type MDNSPacket struct {
	SrcAddr, DstAddr string
	SrcPort, DstPort int
	DNS              dns.Msg
}

func CaptureMDNS() (<-chan MDNSPacket, error) {
	out := make(chan MDNSPacket, 1000)
	handle, err := pcap.OpenLive("any", 1600, true, pcap.BlockForever)
	if err != nil {
		return nil, err
	}

	// for only multicast: udp port 5353 and (dst host 224.0.0.251 or dst host ff02::fb)
	err = handle.SetBPFFilter("udp port 5353")
	if err != nil {
		return nil, err
	}

	go func() {
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		for packet := range packetSource.Packets() {
			var res MDNSPacket

			if ipv4Layer := packet.Layer(layers.LayerTypeIPv4); ipv4Layer != nil {
				ip, _ := ipv4Layer.(*layers.IPv4)
				res.SrcAddr = ip.SrcIP.String()
				res.DstAddr = ip.DstIP.String()
			} else if ipv6Layer := packet.Layer(layers.LayerTypeIPv6); ipv6Layer != nil {
				ip, _ := ipv6Layer.(*layers.IPv6)
				res.SrcAddr = ip.SrcIP.String()
				res.DstAddr = ip.DstIP.String()
			} else {
				continue
			}

			if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
				udp, _ := udpLayer.(*layers.UDP)
				res.SrcPort = int(udp.SrcPort)
				res.DstPort = int(udp.DstPort)

				if msg, err := ParsePacket(udp.Payload); err == nil {
					res.DNS = msg
				} else {
					fmt.Printf("ERROR: malformed packet: %v\n", err)
					continue
				}
			} else {
				continue
			}

			out <- res
		}
	}()

	return out, nil
}

func SplitHost(host string) []string {
	return strings.Split(strings.TrimRight(host, "."), ".")
}

func HostMatches(a, b string) bool {
	aParts := SplitHost(a)
	bParts := SplitHost(b)

	// Early out to catch things like a.b.c.d ~= **.a.b.c.d
	if len(aParts) < len(bParts) {
		return false
	}

	if strings.HasPrefix(b, "**") {
		numWildcards := len(aParts) - (len(bParts) - 1)
		rest := bParts[1:]
		bParts = []string{}
		for i := 0; i < numWildcards; i++ {
			bParts = append(bParts, "*")
		}
		bParts = append(bParts, rest...)
	}
	if strings.HasSuffix(b, "**") {
		numWildcards := len(aParts) - (len(bParts) - 1)
		bParts = bParts[:len(bParts)-1]
		for i := 0; i < numWildcards; i++ {
			bParts = append(bParts, "*")
		}
	}

	if len(aParts) != len(bParts) {
		return false
	}
	for i := range aParts {
		if aParts[i] != bParts[i] && bParts[i] != "*" {
			return false
		}
	}
	return true
}
