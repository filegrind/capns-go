package bifaci

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/filegrind/capns-go/urn"
)

// RelaySwitchError represents errors from relay switch operations
type RelaySwitchError struct {
	Type    RelaySwitchErrorType
	Message string
}

type RelaySwitchErrorType int

const (
	RelaySwitchErrorTypeCbor RelaySwitchErrorType = iota
	RelaySwitchErrorTypeIO
	RelaySwitchErrorTypeNoHandler
	RelaySwitchErrorTypeUnknownRequest
	RelaySwitchErrorTypeProtocol
	RelaySwitchErrorTypeAllMastersUnhealthy
)

func (e *RelaySwitchError) Error() string {
	switch e.Type {
	case RelaySwitchErrorTypeCbor:
		return fmt.Sprintf("relay switch CBOR error: %s", e.Message)
	case RelaySwitchErrorTypeIO:
		return fmt.Sprintf("relay switch I/O error: %s", e.Message)
	case RelaySwitchErrorTypeNoHandler:
		return fmt.Sprintf("no handler found for cap: %s", e.Message)
	case RelaySwitchErrorTypeUnknownRequest:
		return fmt.Sprintf("unknown request ID: %s", e.Message)
	case RelaySwitchErrorTypeProtocol:
		return fmt.Sprintf("protocol violation: %s", e.Message)
	case RelaySwitchErrorTypeAllMastersUnhealthy:
		return "all masters are unhealthy"
	default:
		return fmt.Sprintf("relay switch error: %s", e.Message)
	}
}

// RoutingEntry tracks request source and destination
type RoutingEntry struct {
	SourceMasterIdx      int
	DestinationMasterIdx int
}

// MasterConnection represents a connection to a single RelayMaster
type MasterConnection struct {
	socketWriter *FrameWriter
	manifest     []byte
	limits       Limits
	caps         []string
	healthy      bool
}

// RelaySwitch is a cap-aware routing multiplexer for multiple RelayMasters
type RelaySwitch struct {
	masters          []*MasterConnection
	capTable         []CapTableEntry
	requestRouting   map[string]*RoutingEntry
	peerRequests     map[string]bool
	capabilities     []byte
	negotiatedLimits Limits
	frameRx          chan MasterFrame
	mu               sync.Mutex
}

type CapTableEntry struct {
	CapURN    string
	MasterIdx int
}

type MasterFrame struct {
	MasterIdx int
	Frame     *Frame
	Err       error
}

// ENGINE_SOURCE sentinel value for engine-initiated requests
const ENGINE_SOURCE = -1

// NewRelaySwitch creates a new RelaySwitch with the given socket pairs
func NewRelaySwitch(sockets []SocketPair) (*RelaySwitch, error) {
	if len(sockets) == 0 {
		return nil, &RelaySwitchError{
			Type:    RelaySwitchErrorTypeProtocol,
			Message: "RelaySwitch requires at least one master",
		}
	}

	frameRx := make(chan MasterFrame, 100)
	var masters []*MasterConnection

	// Connect to all masters and spawn reader goroutines
	for masterIdx, sockPair := range sockets {
		socketReader := NewFrameReader(sockPair.Read)
		socketWriter := NewFrameWriter(sockPair.Write)

		// Perform handshake (read initial RelayNotify)
		frame, err := socketReader.ReadFrame()
		if err != nil {
			return nil, err
		}
		if frame == nil {
			return nil, &RelaySwitchError{
				Type:    RelaySwitchErrorTypeProtocol,
				Message: "relay connection closed before receiving RelayNotify",
			}
		}
		if frame.FrameType != FrameTypeRelayNotify {
			return nil, &RelaySwitchError{
				Type:    RelaySwitchErrorTypeProtocol,
				Message: fmt.Sprintf("expected RelayNotify, got %d", frame.FrameType),
			}
		}

		manifest := frame.RelayNotifyManifest()
		if manifest == nil {
			return nil, &RelaySwitchError{
				Type:    RelaySwitchErrorTypeProtocol,
				Message: "RelayNotify missing manifest",
			}
		}

		limits := frame.RelayNotifyLimits()
		if limits == nil {
			return nil, &RelaySwitchError{
				Type:    RelaySwitchErrorTypeProtocol,
				Message: "RelayNotify missing limits",
			}
		}

		caps, err := parseCapabilitiesFromManifest(manifest)
		if err != nil {
			return nil, err
		}

		// Spawn reader goroutine
		idx := masterIdx
		go func() {
			for {
				frame, err := socketReader.ReadFrame()
				if err != nil || frame == nil {
					if err != nil {
						frameRx <- MasterFrame{MasterIdx: idx, Frame: nil, Err: err}
					}
					return
				}

				// Intercept RelayNotify
				if frame.FrameType == FrameTypeRelayNotify {
					// TODO: handle dynamic updates
					continue
				}

				frameRx <- MasterFrame{MasterIdx: idx, Frame: frame, Err: nil}
			}
		}()

		masters = append(masters, &MasterConnection{
			socketWriter: socketWriter,
			manifest:     manifest,
			limits:       *limits,
			caps:         caps,
			healthy:      true,
		})
	}

	sw := &RelaySwitch{
		masters:        masters,
		capTable:       []CapTableEntry{},
		requestRouting: make(map[string]*RoutingEntry),
		peerRequests:   make(map[string]bool),
		frameRx:        frameRx,
	}

	sw.rebuildCapTable()
	sw.rebuildCapabilities()
	sw.rebuildLimits()

	return sw, nil
}

type SocketPair struct {
	Read  net.Conn
	Write net.Conn
}

// Capabilities returns the aggregate capabilities of all healthy masters
func (sw *RelaySwitch) Capabilities() []byte {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	result := make([]byte, len(sw.capabilities))
	copy(result, sw.capabilities)
	return result
}

// Limits returns the negotiated limits (minimum across all masters)
func (sw *RelaySwitch) Limits() Limits {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.negotiatedLimits
}

// SendToMaster sends a frame to the appropriate master
func (sw *RelaySwitch) SendToMaster(frame *Frame) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	switch frame.FrameType {
	case FrameTypeReq:
		if frame.Cap == nil {
			return &RelaySwitchError{
				Type:    RelaySwitchErrorTypeProtocol,
				Message: "REQ frame missing cap URN",
			}
		}

		destIdx, err := sw.findMasterForCap(*frame.Cap)
		if err != nil {
			return err
		}

		sw.requestRouting[frame.Id.ToString()] = &RoutingEntry{
			SourceMasterIdx:      ENGINE_SOURCE,
			DestinationMasterIdx: destIdx,
		}

		return sw.masters[destIdx].socketWriter.WriteFrame(frame)

	case FrameTypeStreamStart, FrameTypeChunk, FrameTypeStreamEnd,
		FrameTypeEnd, FrameTypeErr:
		entry, ok := sw.requestRouting[frame.Id.ToString()]
		if !ok {
			return &RelaySwitchError{
				Type:    RelaySwitchErrorTypeUnknownRequest,
				Message: frame.Id.ToString(),
			}
		}

		destIdx := entry.DestinationMasterIdx
		err := sw.masters[destIdx].socketWriter.WriteFrame(frame)
		if err != nil {
			return err
		}

		// Cleanup on terminal frames for peer responses
		isTerminal := frame.FrameType == FrameTypeEnd || frame.FrameType == FrameTypeErr
		if isTerminal && sw.peerRequests[frame.Id.ToString()] {
			delete(sw.requestRouting, frame.Id.ToString())
			delete(sw.peerRequests, frame.Id.ToString())
		}

		return nil

	default:
		return &RelaySwitchError{
			Type:    RelaySwitchErrorTypeProtocol,
			Message: fmt.Sprintf("unexpected frame type from engine: %d", frame.FrameType),
		}
	}
}

// ReadFromMasters blocks until a frame is available from any master
func (sw *RelaySwitch) ReadFromMasters() (*Frame, error) {
	for {
		masterFrame := <-sw.frameRx

		if masterFrame.Err != nil {
			sw.mu.Lock()
			sw.handleMasterDeath(masterFrame.MasterIdx)
			sw.mu.Unlock()
			continue
		}

		if masterFrame.Frame == nil {
			// EOF
			sw.mu.Lock()
			sw.handleMasterDeath(masterFrame.MasterIdx)
			sw.mu.Unlock()
			continue
		}

		sw.mu.Lock()
		resultFrame, err := sw.handleMasterFrame(masterFrame.MasterIdx, masterFrame.Frame)
		sw.mu.Unlock()

		if err != nil {
			return nil, err
		}

		if resultFrame != nil {
			return resultFrame, nil
		}
		// Peer request handled internally, continue reading
	}
}

// findMasterForCap finds which master handles a given cap URN
func (sw *RelaySwitch) findMasterForCap(capURN string) (int, error) {
	// Try exact match first
	for _, entry := range sw.capTable {
		if entry.CapURN == capURN {
			return entry.MasterIdx, nil
		}
	}

	// Try URN-level matching
	requestURN, err := urn.NewCapUrnFromString(capURN)
	if err == nil {
		for _, entry := range sw.capTable {
			registeredURN, err := urn.NewCapUrnFromString(entry.CapURN)
			if err == nil {
				if requestURN.Accepts(registeredURN) {
					return entry.MasterIdx, nil
				}
			}
		}
	}

	return 0, &RelaySwitchError{
		Type:    RelaySwitchErrorTypeNoHandler,
		Message: capURN,
	}
}

// handleMasterFrame handles a frame from a master
func (sw *RelaySwitch) handleMasterFrame(sourceIdx int, frame *Frame) (*Frame, error) {
	switch frame.FrameType {
	case FrameTypeReq:
		// Peer request
		if frame.Cap == nil {
			return nil, &RelaySwitchError{
				Type:    RelaySwitchErrorTypeProtocol,
				Message: "REQ frame missing cap URN",
			}
		}

		destIdx, err := sw.findMasterForCap(*frame.Cap)
		if err != nil {
			return nil, err
		}

		sw.requestRouting[frame.Id.ToString()] = &RoutingEntry{
			SourceMasterIdx:      sourceIdx,
			DestinationMasterIdx: destIdx,
		}
		sw.peerRequests[frame.Id.ToString()] = true

		err = sw.masters[destIdx].socketWriter.WriteFrame(frame)
		if err != nil {
			return nil, err
		}

		return nil, nil // Internal routing

	case FrameTypeStreamStart, FrameTypeChunk, FrameTypeStreamEnd,
		FrameTypeEnd, FrameTypeErr, FrameTypeLog:
		entry, ok := sw.requestRouting[frame.Id.ToString()]
		if ok && entry.SourceMasterIdx != ENGINE_SOURCE {
			// Response to peer request
			destIdx := entry.SourceMasterIdx
			isTerminal := frame.FrameType == FrameTypeEnd || frame.FrameType == FrameTypeErr

			err := sw.masters[destIdx].socketWriter.WriteFrame(frame)
			if err != nil {
				return nil, err
			}

			if isTerminal && !sw.peerRequests[frame.Id.ToString()] {
				delete(sw.requestRouting, frame.Id.ToString())
			}

			return nil, nil
		}

		// Response to engine request
		isTerminal := frame.FrameType == FrameTypeEnd || frame.FrameType == FrameTypeErr
		if isTerminal && !sw.peerRequests[frame.Id.ToString()] {
			delete(sw.requestRouting, frame.Id.ToString())
		}

		return frame, nil

	default:
		return frame, nil
	}
}

// handleMasterDeath handles master death
func (sw *RelaySwitch) handleMasterDeath(masterIdx int) {
	if !sw.masters[masterIdx].healthy {
		return
	}

	sw.masters[masterIdx].healthy = false

	// Cleanup routing
	for reqID, entry := range sw.requestRouting {
		if entry.DestinationMasterIdx == masterIdx {
			delete(sw.requestRouting, reqID)
			delete(sw.peerRequests, reqID)
		}
	}

	sw.rebuildCapTable()
	sw.rebuildCapabilities()
	sw.rebuildLimits()
}

// rebuildCapTable rebuilds the cap table from all healthy masters
func (sw *RelaySwitch) rebuildCapTable() {
	sw.capTable = []CapTableEntry{}
	for idx, master := range sw.masters {
		if master.healthy {
			for _, cap := range master.caps {
				sw.capTable = append(sw.capTable, CapTableEntry{
					CapURN:    cap,
					MasterIdx: idx,
				})
			}
		}
	}
}

// rebuildCapabilities rebuilds aggregate capabilities
func (sw *RelaySwitch) rebuildCapabilities() {
	capSet := make(map[string]bool)
	for _, master := range sw.masters {
		if master.healthy {
			for _, cap := range master.caps {
				capSet[cap] = true
			}
		}
	}

	caps := []string{}
	for cap := range capSet {
		caps = append(caps, cap)
	}

	manifest := map[string]interface{}{
		"capabilities": caps,
	}
	data, _ := json.Marshal(manifest)
	sw.capabilities = data
}

// rebuildLimits rebuilds negotiated limits
func (sw *RelaySwitch) rebuildLimits() {
	minFrame := int(^uint(0) >> 1) // Max int
	minChunk := int(^uint(0) >> 1)

	for _, master := range sw.masters {
		if master.healthy {
			if master.limits.MaxFrame < minFrame {
				minFrame = master.limits.MaxFrame
			}
			if master.limits.MaxChunk < minChunk {
				minChunk = master.limits.MaxChunk
			}
		}
	}

	if minFrame == int(^uint(0)>>1) {
		minFrame = DefaultMaxFrame
	}
	if minChunk == int(^uint(0)>>1) {
		minChunk = DefaultMaxChunk
	}

	sw.negotiatedLimits = Limits{
		MaxFrame: minFrame,
		MaxChunk: minChunk,
	}
}

// parseCapabilitiesFromManifest parses capability URNs from manifest JSON
func parseCapabilitiesFromManifest(manifest []byte) ([]string, error) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(manifest, &parsed); err != nil {
		return nil, &RelaySwitchError{
			Type:    RelaySwitchErrorTypeProtocol,
			Message: fmt.Sprintf("invalid manifest JSON: %v", err),
		}
	}

	capsIface, ok := parsed["capabilities"]
	if !ok {
		return nil, &RelaySwitchError{
			Type:    RelaySwitchErrorTypeProtocol,
			Message: "manifest missing capabilities array",
		}
	}

	capsArray, ok := capsIface.([]interface{})
	if !ok {
		return nil, &RelaySwitchError{
			Type:    RelaySwitchErrorTypeProtocol,
			Message: "capabilities is not an array",
		}
	}

	caps := []string{}
	for _, capIface := range capsArray {
		cap, ok := capIface.(string)
		if !ok {
			return nil, &RelaySwitchError{
				Type:    RelaySwitchErrorTypeProtocol,
				Message: "non-string capability",
			}
		}
		caps = append(caps, cap)
	}

	return caps, nil
}
