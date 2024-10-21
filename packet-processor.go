// packet_processor.go
package main

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"
)

type PacketProcessor struct {
	handle       *pcap.Handle
	assembler    *tcpassembly.Assembler
	packetSource *gopacket.PacketSource
}

func NewPacketProcessor(device string, assembler *tcpassembly.Assembler) (*PacketProcessor, error) {
	handle, err := pcap.OpenLive(device, 65536, true, pcap.BlockForever)
	if err != nil {
		return nil, err
	}
	err = handle.SetBPFFilter("tcp port 80")
	if err != nil {
		return nil, err
	}
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	return &PacketProcessor{
		handle:       handle,
		assembler:    assembler,
		packetSource: packetSource,
	}, nil
}

func (p *PacketProcessor) StartCapture() {
	packets := p.packetSource.Packets()
	for packet := range packets {
		if tcp, ok := packet.Layer(layers.LayerTypeTCP).(*layers.TCP); ok {
			if tcp.DstPort == 80 || tcp.SrcPort == 80 {
				p.assembler.AssembleWithTimestamp(packet.NetworkLayer().NetworkFlow(), tcp, packet.Metadata().Timestamp)
			}
		}
	}
}

func (p *PacketProcessor) Close() {
	p.handle.Close()
}
