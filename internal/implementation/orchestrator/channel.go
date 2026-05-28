package orchestrator

import "github.com/wmulabs/eywa/internal/domain/ports"

// Channel pairs an inbound Receptor with an outbound Voice under a single name.
// When a Link sets ChannelName, both InboundConverterName and VoiceName resolve to this name.
type Channel struct {
	Name     string
	Receptor ports.Receptor
	Voice    ports.Voice
}
