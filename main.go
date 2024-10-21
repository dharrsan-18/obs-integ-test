package main

import (
	"log"

	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"
)

func main() {
	// Find the device
	devices, err := pcap.FindAllDevs()
	if err != nil {
		log.Fatal(err)
	}
	if len(devices) == 0 {
		log.Fatal("No devices found")
	}

	// Choose the first device
	device := devices[0].Name
	// You might need to adjust the device name based on your system

	streamFactory := &HttpStreamFactory{}
	streamPool := tcpassembly.NewStreamPool(streamFactory)
	assembler := tcpassembly.NewAssembler(streamPool)

	packetProcessor, err := NewPacketProcessor(device, assembler)
	if err != nil {
		log.Fatal(err)
	}
	defer packetProcessor.Close()

	packetProcessor.StartCapture() // Pass otelExporter to StartCapture
}
