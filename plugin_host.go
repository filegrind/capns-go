package capns

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/filegrind/capns-go/cbor"
)

// CapHandler is a function that handles a peer invoke request.
// It receives the concatenated payload bytes and returns response bytes.
type CapHandler func(payload []byte) ([]byte, error)

// ResponseChunk represents a response chunk from a plugin (matches Rust ResponseChunk)
type ResponseChunk struct {
	Payload []byte
	Seq     uint64
	Offset  *uint64
	Len     *uint64
	IsEof   bool
}

// PluginResponseType indicates whether a response is single or streaming
type PluginResponseType int

const (
	PluginResponseTypeSingle    PluginResponseType = iota
	PluginResponseTypeStreaming
)

// PluginResponse represents a complete response from a plugin
type PluginResponse struct {
	Type      PluginResponseType
	Single    []byte
	Streaming []*ResponseChunk
}

// FinalPayload gets the final payload
func (pr *PluginResponse) FinalPayload() []byte {
	switch pr.Type {
	case PluginResponseTypeSingle:
		return pr.Single
	case PluginResponseTypeStreaming:
		if len(pr.Streaming) > 0 {
			return pr.Streaming[len(pr.Streaming)-1].Payload
		}
		return nil
	default:
		return nil
	}
}

// Concatenated concatenates all payloads into a single buffer
func (pr *PluginResponse) Concatenated() []byte {
	switch pr.Type {
	case PluginResponseTypeSingle:
		result := make([]byte, len(pr.Single))
		copy(result, pr.Single)
		return result
	case PluginResponseTypeStreaming:
		totalLen := 0
		for _, chunk := range pr.Streaming {
			totalLen += len(chunk.Payload)
		}
		result := make([]byte, 0, totalLen)
		for _, chunk := range pr.Streaming {
			result = append(result, chunk.Payload...)
		}
		return result
	default:
		return nil
	}
}

// HostError represents errors from the plugin host
type HostError struct {
	Type    HostErrorType
	Message string
	Code    string
}

type HostErrorType int

const (
	HostErrorTypeCbor HostErrorType = iota
	HostErrorTypeIo
	HostErrorTypePluginError
	HostErrorTypeUnexpectedFrameType
	HostErrorTypeProcessExited
	HostErrorTypeHandshake
	HostErrorTypeClosed
	HostErrorTypeSendError
	HostErrorTypeRecvError
)

func (e *HostError) Error() string {
	switch e.Type {
	case HostErrorTypeCbor:
		return fmt.Sprintf("CBOR error: %s", e.Message)
	case HostErrorTypeIo:
		return fmt.Sprintf("I/O error: %s", e.Message)
	case HostErrorTypePluginError:
		return fmt.Sprintf("Plugin returned error: [%s] %s", e.Code, e.Message)
	case HostErrorTypeUnexpectedFrameType:
		return fmt.Sprintf("Unexpected frame type: %s", e.Message)
	case HostErrorTypeProcessExited:
		return "Plugin process exited unexpectedly"
	case HostErrorTypeHandshake:
		return fmt.Sprintf("Handshake failed: %s", e.Message)
	case HostErrorTypeClosed:
		return "Host is closed"
	case HostErrorTypeSendError:
		return "Send error: channel closed"
	case HostErrorTypeRecvError:
		return "Receive error: channel closed"
	default:
		return fmt.Sprintf("Unknown error: %s", e.Message)
	}
}

// =========================================================================
// Multi-plugin host
// =========================================================================

// pluginEvent is an internal event from a plugin reader goroutine.
type pluginEvent struct {
	pluginIdx int
	frame     *cbor.Frame
	isDeath   bool
}

// capTableEntry maps a cap URN to a plugin index.
type capTableEntry struct {
	capUrn    string
	pluginIdx int
}

// routingEntry tracks a routed request with its original MessageId.
type routingEntry struct {
	pluginIdx int
	msgId     cbor.MessageId
}

// ManagedPlugin represents a plugin managed by the PluginHost.
type ManagedPlugin struct {
	path        string
	cmd         *exec.Cmd
	writerCh    chan *cbor.Frame
	manifest    []byte
	limits      cbor.Limits
	caps        []string
	knownCaps   []string
	running     bool
	helloFailed bool
}

// PluginHost manages N plugin binaries with cap-based routing.
//
// Plugins are either registered (for on-demand spawning) or attached
// (pre-connected). REQ frames from the relay are routed to the correct
// plugin by cap URN. Continuation frames (STREAM_START, CHUNK,
// STREAM_END, END) are routed by request ID.
type PluginHost struct {
	plugins        []*ManagedPlugin
	capTable       []capTableEntry
	requestRouting map[string]routingEntry // reqId string → routing info
	peerRequests   map[string]bool         // plugin-initiated reqIds
	capabilities   []byte
	eventCh        chan pluginEvent
	mu             sync.Mutex
}

// NewPluginHost creates a new multi-plugin host.
func NewPluginHost() *PluginHost {
	return &PluginHost{
		requestRouting: make(map[string]routingEntry),
		peerRequests:   make(map[string]bool),
		eventCh:        make(chan pluginEvent, 256),
	}
}

// RegisterPlugin registers a plugin binary for on-demand spawning.
// The plugin is not spawned until a REQ arrives for one of its known caps.
func (h *PluginHost) RegisterPlugin(path string, knownCaps []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	pluginIdx := len(h.plugins)
	h.plugins = append(h.plugins, &ManagedPlugin{
		path:      path,
		knownCaps: knownCaps,
		running:   false,
		limits:    cbor.DefaultLimits(),
	})

	for _, cap := range knownCaps {
		h.capTable = append(h.capTable, capTableEntry{capUrn: cap, pluginIdx: pluginIdx})
	}
}

// AttachPlugin attaches a pre-connected plugin (already running).
// Performs HELLO handshake immediately and returns the plugin index.
func (h *PluginHost) AttachPlugin(pluginRead io.Reader, pluginWrite io.Writer) (int, error) {
	reader := cbor.NewFrameReader(pluginRead)
	writer := cbor.NewFrameWriter(pluginWrite)

	manifest, limits, err := cbor.HandshakeInitiate(reader, writer)
	if err != nil {
		return -1, fmt.Errorf("handshake failed: %w", err)
	}

	reader.SetLimits(limits)
	writer.SetLimits(limits)

	caps, err := parseCapsFromManifest(manifest)
	if err != nil {
		return -1, fmt.Errorf("failed to parse manifest: %w", err)
	}

	h.mu.Lock()
	pluginIdx := len(h.plugins)

	writerCh := make(chan *cbor.Frame, 64)
	plugin := &ManagedPlugin{
		writerCh: writerCh,
		manifest: manifest,
		limits:   limits,
		caps:     caps,
		running:  true,
	}
	h.plugins = append(h.plugins, plugin)

	for _, cap := range caps {
		h.capTable = append(h.capTable, capTableEntry{capUrn: cap, pluginIdx: pluginIdx})
	}
	h.rebuildCapabilities()
	h.mu.Unlock()

	go h.writerLoop(writer, writerCh)
	go h.readerLoop(pluginIdx, reader)

	return pluginIdx, nil
}

// Capabilities returns the aggregate capabilities of all running plugins as JSON.
func (h *PluginHost) Capabilities() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.capabilities
}

// FindPluginForCap finds the plugin index that can handle a given cap URN.
// Returns (pluginIdx, true) if found, (-1, false) if not.
func (h *PluginHost) FindPluginForCap(capUrn string) (int, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.findPluginForCapLocked(capUrn)
}

func (h *PluginHost) findPluginForCapLocked(capUrn string) (int, bool) {
	// Exact match first
	for _, entry := range h.capTable {
		if entry.capUrn == capUrn {
			return entry.pluginIdx, true
		}
	}

	// URN-level matching: request is pattern, registered cap is instance
	requestUrn, err := NewCapUrnFromString(capUrn)
	if err != nil {
		return -1, false
	}

	for _, entry := range h.capTable {
		registeredUrn, err := NewCapUrnFromString(entry.capUrn)
		if err != nil {
			continue
		}
		if requestUrn.Accepts(registeredUrn) {
			return entry.pluginIdx, true
		}
	}

	return -1, false
}

// Run runs the main event loop, reading from relay and plugins.
// Blocks until relay closes or a fatal error occurs.
func (h *PluginHost) Run(relayRead io.Reader, relayWrite io.Writer, resourceFn func() []byte) error {
	relayReader := cbor.NewFrameReader(relayRead)
	relayWriter := cbor.NewFrameWriter(relayWrite)

	relayCh := make(chan *cbor.Frame, 64)
	relayDone := make(chan error, 1)
	go func() {
		for {
			frame, err := relayReader.ReadFrame()
			if err != nil {
				if err == io.EOF {
					relayDone <- nil
				} else {
					relayDone <- err
				}
				close(relayCh)
				return
			}
			relayCh <- frame
		}
	}()

	for {
		select {
		case frame, ok := <-relayCh:
			if !ok {
				err := <-relayDone
				h.killAllPlugins()
				return err
			}
			if err := h.handleRelayFrame(frame, relayWriter); err != nil {
				h.killAllPlugins()
				return err
			}

		case event := <-h.eventCh:
			if event.isDeath {
				h.handlePluginDeath(event.pluginIdx, relayWriter)
			} else if event.frame != nil {
				h.handlePluginFrame(event.pluginIdx, event.frame, relayWriter)
			}
		}
	}
}

// handleRelayFrame routes an incoming frame from the relay to the correct plugin.
func (h *PluginHost) handleRelayFrame(frame *cbor.Frame, relayWriter *cbor.FrameWriter) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	idKey := frame.Id.ToString()

	switch frame.FrameType {
	case cbor.FrameTypeReq:
		capUrn := ""
		if frame.Cap != nil {
			capUrn = *frame.Cap
		}

		pluginIdx, found := h.findPluginForCapLocked(capUrn)
		if !found {
			errFrame := cbor.NewErr(frame.Id, "NO_HANDLER", fmt.Sprintf("no plugin handles cap: %s", capUrn))
			relayWriter.WriteFrame(errFrame)
			return nil
		}

		plugin := h.plugins[pluginIdx]
		if !plugin.running {
			if plugin.helloFailed {
				errFrame := cbor.NewErr(frame.Id, "SPAWN_FAILED", "plugin previously failed to start")
				relayWriter.WriteFrame(errFrame)
				return nil
			}
			if err := h.spawnPluginLocked(pluginIdx); err != nil {
				errFrame := cbor.NewErr(frame.Id, "SPAWN_FAILED", err.Error())
				relayWriter.WriteFrame(errFrame)
				return nil
			}
		}

		h.requestRouting[idKey] = routingEntry{pluginIdx: pluginIdx, msgId: frame.Id}
		h.sendToPlugin(pluginIdx, frame)

	case cbor.FrameTypeStreamStart, cbor.FrameTypeChunk, cbor.FrameTypeStreamEnd:
		if entry, ok := h.requestRouting[idKey]; ok {
			h.sendToPlugin(entry.pluginIdx, frame)
		}

	case cbor.FrameTypeEnd, cbor.FrameTypeErr:
		if entry, ok := h.requestRouting[idKey]; ok {
			h.sendToPlugin(entry.pluginIdx, frame)
			// Only remove routing on terminal frames if this is a PEER response
			// (engine responding to a plugin's peer invoke). For engine-initiated
			// requests, the relay END is just the end of the request body — the
			// plugin still needs to respond, so routing must survive.
			if h.peerRequests[idKey] {
				delete(h.requestRouting, idKey)
				delete(h.peerRequests, idKey)
			}
		}

	case cbor.FrameTypeHeartbeat:
		// Engine-level heartbeat — not forwarded to plugins
		return nil

	case cbor.FrameTypeHello:
		return fmt.Errorf("unexpected HELLO from relay")

	case cbor.FrameTypeRelayNotify, cbor.FrameTypeRelayState:
		return fmt.Errorf("relay frame %v reached plugin host", frame.FrameType)
	}

	return nil
}

// handlePluginFrame processes a frame from a plugin.
func (h *PluginHost) handlePluginFrame(pluginIdx int, frame *cbor.Frame, relayWriter *cbor.FrameWriter) {
	h.mu.Lock()
	defer h.mu.Unlock()

	idKey := frame.Id.ToString()

	switch frame.FrameType {
	case cbor.FrameTypeHeartbeat:
		// Respond to plugin heartbeat locally — don't forward
		response := cbor.NewHeartbeat(frame.Id)
		h.sendToPlugin(pluginIdx, response)

	case cbor.FrameTypeHello:
		// HELLO post-handshake — protocol violation, ignore
		return

	case cbor.FrameTypeReq:
		// Plugin is invoking a peer cap (sending request to engine)
		h.requestRouting[idKey] = routingEntry{pluginIdx: pluginIdx, msgId: frame.Id}
		h.peerRequests[idKey] = true
		relayWriter.WriteFrame(frame)

	case cbor.FrameTypeLog:
		relayWriter.WriteFrame(frame)

	case cbor.FrameTypeStreamStart, cbor.FrameTypeChunk, cbor.FrameTypeStreamEnd:
		relayWriter.WriteFrame(frame)

	case cbor.FrameTypeEnd:
		relayWriter.WriteFrame(frame)
		if !h.peerRequests[idKey] {
			delete(h.requestRouting, idKey)
		}

	case cbor.FrameTypeErr:
		relayWriter.WriteFrame(frame)
		delete(h.requestRouting, idKey)
		delete(h.peerRequests, idKey)
	}
}

// handlePluginDeath processes a plugin death event.
func (h *PluginHost) handlePluginDeath(pluginIdx int, relayWriter *cbor.FrameWriter) {
	h.mu.Lock()
	defer h.mu.Unlock()

	plugin := h.plugins[pluginIdx]
	plugin.running = false

	if plugin.writerCh != nil {
		close(plugin.writerCh)
		plugin.writerCh = nil
	}

	if plugin.cmd != nil && plugin.cmd.Process != nil {
		plugin.cmd.Process.Kill()
		plugin.cmd = nil
	}

	// Send ERR for all pending requests routed to this plugin
	var failedEntries []routingEntry
	var failedKeys []string
	for reqId, entry := range h.requestRouting {
		if entry.pluginIdx == pluginIdx {
			failedEntries = append(failedEntries, entry)
			failedKeys = append(failedKeys, reqId)
		}
	}

	for i, key := range failedKeys {
		errFrame := cbor.NewErr(failedEntries[i].msgId, "PLUGIN_DIED", fmt.Sprintf("plugin %d died", pluginIdx))
		relayWriter.WriteFrame(errFrame)
		delete(h.requestRouting, key)
		delete(h.peerRequests, key)
	}

	h.updateCapTable()
	h.rebuildCapabilities()
}

// sendToPlugin sends a frame to a plugin via its writer channel.
func (h *PluginHost) sendToPlugin(pluginIdx int, frame *cbor.Frame) {
	plugin := h.plugins[pluginIdx]
	if plugin.writerCh != nil {
		select {
		case plugin.writerCh <- frame:
		default:
			// Channel full — plugin probably dead, frame dropped
		}
	}
}

// writerLoop reads frames from the channel and writes them to the plugin.
func (h *PluginHost) writerLoop(writer *cbor.FrameWriter, ch chan *cbor.Frame) {
	for frame := range ch {
		if err := writer.WriteFrame(frame); err != nil {
			return
		}
	}
}

// readerLoop reads frames from a plugin and sends events to the event channel.
func (h *PluginHost) readerLoop(pluginIdx int, reader *cbor.FrameReader) {
	for {
		frame, err := reader.ReadFrame()
		if err != nil {
			h.eventCh <- pluginEvent{pluginIdx: pluginIdx, isDeath: true}
			return
		}
		h.eventCh <- pluginEvent{pluginIdx: pluginIdx, frame: frame}
	}
}

// spawnPluginLocked spawns a registered plugin process (caller must hold mu).
func (h *PluginHost) spawnPluginLocked(pluginIdx int) error {
	plugin := h.plugins[pluginIdx]
	if plugin.path == "" {
		plugin.helloFailed = true
		return fmt.Errorf("plugin has no path")
	}

	cmd := exec.Command(plugin.path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		plugin.helloFailed = true
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		plugin.helloFailed = true
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		plugin.helloFailed = true
		return fmt.Errorf("failed to start plugin: %w", err)
	}
	plugin.cmd = cmd

	reader := cbor.NewFrameReader(stdout)
	writer := cbor.NewFrameWriter(stdin)

	manifest, limits, err := cbor.HandshakeInitiate(reader, writer)
	if err != nil {
		plugin.helloFailed = true
		cmd.Process.Kill()
		return fmt.Errorf("handshake failed: %w", err)
	}

	reader.SetLimits(limits)
	writer.SetLimits(limits)

	caps, parseErr := parseCapsFromManifest(manifest)
	if parseErr != nil {
		plugin.helloFailed = true
		cmd.Process.Kill()
		return fmt.Errorf("failed to parse manifest: %w", parseErr)
	}

	plugin.manifest = manifest
	plugin.limits = limits
	plugin.caps = caps
	plugin.running = true

	writerCh := make(chan *cbor.Frame, 64)
	plugin.writerCh = writerCh

	h.updateCapTable()
	h.rebuildCapabilities()

	go h.writerLoop(writer, writerCh)
	go h.readerLoop(pluginIdx, reader)

	return nil
}

// updateCapTable rebuilds the cap table from all plugins.
func (h *PluginHost) updateCapTable() {
	h.capTable = nil
	for idx, plugin := range h.plugins {
		if plugin.helloFailed {
			continue
		}
		caps := plugin.knownCaps
		if plugin.running && len(plugin.caps) > 0 {
			caps = plugin.caps
		}
		for _, cap := range caps {
			h.capTable = append(h.capTable, capTableEntry{capUrn: cap, pluginIdx: idx})
		}
	}
}

// rebuildCapabilities rebuilds the aggregate capabilities JSON.
func (h *PluginHost) rebuildCapabilities() {
	var allCaps []string
	for _, plugin := range h.plugins {
		if plugin.running {
			allCaps = append(allCaps, plugin.caps...)
		}
	}

	if len(allCaps) == 0 {
		h.capabilities = nil
		return
	}

	capsJSON, err := json.Marshal(map[string]interface{}{
		"caps": allCaps,
	})
	if err != nil {
		h.capabilities = nil
		return
	}
	h.capabilities = capsJSON
}

// killAllPlugins stops all managed plugins.
func (h *PluginHost) killAllPlugins() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, plugin := range h.plugins {
		if plugin.writerCh != nil {
			close(plugin.writerCh)
			plugin.writerCh = nil
		}
		if plugin.cmd != nil && plugin.cmd.Process != nil {
			plugin.cmd.Process.Kill()
		}
		plugin.running = false
	}
}

// parseCapsFromManifest parses cap URNs from a JSON manifest.
// Expected format: {"caps": [{"urn": "cap:op=test", ...}, ...]}
func parseCapsFromManifest(manifest []byte) ([]string, error) {
	if len(manifest) == 0 {
		return nil, nil
	}

	var parsed struct {
		Caps []struct {
			Urn string `json:"urn"`
		} `json:"caps"`
	}

	if err := json.Unmarshal(manifest, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse manifest JSON: %w", err)
	}

	var caps []string
	for _, cap := range parsed.Caps {
		if cap.Urn != "" {
			caps = append(caps, cap.Urn)
		}
	}

	return caps, nil
}
