package main

import (
	"fmt"

	dnspacket "github.com/bvisness/buongiorno/src/packet"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

func main() {
	handle, err := pcap.OpenLive("any", 1600, true, pcap.BlockForever)
	if err != nil {
		panic(err)
	}

	err = handle.SetBPFFilter("udp port 5353 and (dst host 224.0.0.251 or dst host ff02::fb)")
	if err != nil {
		panic(err)
	}

	fmt.Printf("Listening...\n")
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
			udp, _ := udpLayer.(*layers.UDP)
			if msg, err := dnspacket.ParsePacket(udp.Payload); err == nil {
				fmt.Println(msg.String())
			} else {
				fmt.Printf("ERROR: malformed packet: %v\n", err)
			}
		}
	}
}
