package capns

import (
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/filegrind/capns-go/cbor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testHostManifest = `{"name":"Test","version":"1.0","caps":[{"urn":"cap:in=media:;out=media:"}]}`

// simulatePlugin runs a fake plugin: handshake + handler on the plugin side of a pipe.
// handler receives the FrameReader/FrameWriter after handshake and can read/write frames.
func simulatePlugin(t *testing.T, pluginRead, pluginWrite net.Conn, manifest string, handler func(*cbor.FrameReader, *cbor.FrameWriter)) {
	t.Helper()
	reader := cbor.NewFrameReader(pluginRead)
	writer := cbor.NewFrameWriter(pluginWrite)

	limits, err := cbor.HandshakeAccept(reader, writer, []byte(manifest))
	require.NoError(t, err)
	reader.SetLimits(limits)
	writer.SetLimits(limits)

	if handler != nil {
		handler(reader, writer)
	}
}

// TEST413: RegisterPlugin adds entries to capTable
func TestRegisterPluginAddsCapTable(t *testing.T) {
	host := NewPluginHost()
	host.RegisterPlugin("/path/to/converter", []string{"cap:op=convert", "cap:op=analyze"})

	host.mu.Lock()
	defer host.mu.Unlock()

	assert.Equal(t, 2, len(host.capTable), "must have 2 cap table entries")
	assert.Equal(t, "cap:op=convert", host.capTable[0].capUrn)
	assert.Equal(t, 0, host.capTable[0].pluginIdx)
	assert.Equal(t, "cap:op=analyze", host.capTable[1].capUrn)
	assert.Equal(t, 0, host.capTable[1].pluginIdx)

	assert.Equal(t, 1, len(host.plugins))
	assert.False(t, host.plugins[0].running, "registered plugin must not be running")
}

// TEST414: Capabilities() returns nil when no plugins are running
func TestCapabilitiesEmptyInitially(t *testing.T) {
	// Case 1: No plugins at all
	host := NewPluginHost()
	assert.Nil(t, host.Capabilities(), "no plugins → nil capabilities")

	// Case 2: Plugin registered but not running
	host.RegisterPlugin("/path/to/plugin", []string{"cap:op=test"})
	assert.Nil(t, host.Capabilities(), "registered but not running → nil capabilities")
}

// TEST415: REQ for known cap triggers spawn (expect error for non-existent binary)
func TestReqTriggersSpawn(t *testing.T) {
	host := NewPluginHost()
	host.RegisterPlugin("/nonexistent/plugin/binary", []string{"cap:op=test"})

	// Set up relay pipes
	relayRead, engineWrite := net.Pipe()
	engineRead, relayWrite := net.Pipe()
	defer relayRead.Close()
	defer relayWrite.Close()

	// Engine sends REQ then closes
	go func() {
		writer := cbor.NewFrameWriter(engineWrite)
		reqId := cbor.NewMessageIdRandom()
		req := cbor.NewReq(reqId, "cap:op=test", []byte("hello"), "text/plain")
		writer.WriteFrame(req)

		// Read the ERR response
		reader := cbor.NewFrameReader(engineRead)
		frame, err := reader.ReadFrame()
		if err == nil {
			assert.Equal(t, cbor.FrameTypeErr, frame.FrameType)
			errCode := frame.ErrorCode()
			assert.Equal(t, "SPAWN_FAILED", errCode, "spawn of nonexistent binary must fail")
		}

		// Close relay to end Run()
		engineWrite.Close()
		engineRead.Close()
	}()

	err := host.Run(relayRead, relayWrite, nil)
	// Run returns when relay closes — nil is normal EOF
	_ = err
}

// TEST416: AttachPlugin performs HELLO handshake, extracts manifest, updates capabilities
func TestAttachPluginHandshake(t *testing.T) {
	manifest := `{"name":"Test","version":"1.0","caps":[{"urn":"cap:in=media:;out=media:"}]}`

	hostRead, pluginWrite := net.Pipe()
	pluginRead, hostWrite := net.Pipe()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		simulatePlugin(t, pluginRead, pluginWrite, manifest, nil)
		pluginRead.Close()
		pluginWrite.Close()
	}()

	host := NewPluginHost()
	idx, err := host.AttachPlugin(hostRead, hostWrite)
	require.NoError(t, err)

	assert.Equal(t, 0, idx, "first attached plugin is index 0")

	host.mu.Lock()
	assert.True(t, host.plugins[0].running, "attached plugin must be running")
	assert.Equal(t, []string{"cap:in=media:;out=media:"}, host.plugins[0].caps)
	host.mu.Unlock()

	caps := host.Capabilities()
	assert.NotNil(t, caps, "running plugin must produce capabilities")
	assert.Contains(t, string(caps), "cap:in=media:;out=media:")

	// Clean up
	hostRead.Close()
	hostWrite.Close()
	wg.Wait()
}

// TEST417: Route REQ to correct plugin by cap_urn (two plugins)
func TestRouteReqByCapUrn(t *testing.T) {
	manifestA := `{"name":"PluginA","version":"1.0","caps":[{"urn":"cap:op=convert"}]}`
	manifestB := `{"name":"PluginB","version":"1.0","caps":[{"urn":"cap:op=analyze"}]}`

	// Plugin A pipes
	hostReadA, pluginWriteA := net.Pipe()
	pluginReadA, hostWriteA := net.Pipe()

	// Plugin B pipes
	hostReadB, pluginWriteB := net.Pipe()
	pluginReadB, hostWriteB := net.Pipe()

	var wg sync.WaitGroup

	// Plugin A: reads REQ+stream, responds with "converted"
	wg.Add(1)
	go func() {
		defer wg.Done()
		simulatePlugin(t, pluginReadA, pluginWriteA, manifestA, func(r *cbor.FrameReader, w *cbor.FrameWriter) {
			// Read REQ
			frame, err := r.ReadFrame()
			require.NoError(t, err)
			assert.Equal(t, cbor.FrameTypeReq, frame.FrameType)
			reqId := frame.Id

			// Read until END
			for {
				f, err := r.ReadFrame()
				if err != nil {
					break
				}
				if f.FrameType == cbor.FrameTypeEnd {
					break
				}
			}

			// Respond
			w.WriteFrame(cbor.NewEnd(reqId, []byte("converted")))
		})
		pluginReadA.Close()
		pluginWriteA.Close()
	}()

	// Plugin B: just does handshake, expects no REQs, waits for close
	wg.Add(1)
	go func() {
		defer wg.Done()
		simulatePlugin(t, pluginReadB, pluginWriteB, manifestB, func(r *cbor.FrameReader, w *cbor.FrameWriter) {
			// Should get EOF (no frames sent to B)
			_, err := r.ReadFrame()
			assert.Error(t, err, "plugin B must get EOF, not a frame")
		})
		pluginReadB.Close()
		pluginWriteB.Close()
	}()

	host := NewPluginHost()
	_, err := host.AttachPlugin(hostReadA, hostWriteA)
	require.NoError(t, err)
	_, err = host.AttachPlugin(hostReadB, hostWriteB)
	require.NoError(t, err)

	// Relay pipes
	relayRead, engineWrite := net.Pipe()
	engineRead, relayWrite := net.Pipe()

	// Engine: send REQ for cap:op=convert, read response
	wg.Add(1)
	go func() {
		defer wg.Done()
		writer := cbor.NewFrameWriter(engineWrite)
		reader := cbor.NewFrameReader(engineRead)

		reqId := cbor.NewMessageIdRandom()
		writer.WriteFrame(cbor.NewReq(reqId, "cap:op=convert", []byte{}, "text/plain"))
		writer.WriteFrame(cbor.NewEnd(reqId, nil))

		// Read response
		frame, err := reader.ReadFrame()
		require.NoError(t, err)
		assert.Equal(t, cbor.FrameTypeEnd, frame.FrameType)
		assert.Equal(t, []byte("converted"), frame.Payload)

		// Close relay
		engineWrite.Close()
		engineRead.Close()
	}()

	host.Run(relayRead, relayWrite, nil)
	relayRead.Close()
	relayWrite.Close()

	// Close host connections to Plugin B to unblock its goroutine
	hostReadB.Close()
	hostWriteB.Close()
	hostReadA.Close()
	hostWriteA.Close()

	wg.Wait()
}

// TEST418: Route STREAM_START/CHUNK/STREAM_END/END by req_id
func TestRouteContinuationByReqId(t *testing.T) {
	manifest := `{"name":"Test","version":"1.0","caps":[{"urn":"cap:op=cont"}]}`

	hostReadP, pluginWriteP := net.Pipe()
	pluginReadP, hostWriteP := net.Pipe()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		simulatePlugin(t, pluginReadP, pluginWriteP, manifest, func(r *cbor.FrameReader, w *cbor.FrameWriter) {
			// Read REQ
			req, err := r.ReadFrame()
			require.NoError(t, err)
			assert.Equal(t, cbor.FrameTypeReq, req.FrameType)
			reqId := req.Id

			// Read STREAM_START
			ss, err := r.ReadFrame()
			require.NoError(t, err)
			assert.Equal(t, cbor.FrameTypeStreamStart, ss.FrameType)
			assert.Equal(t, reqId.ToString(), ss.Id.ToString())

			// Read CHUNK
			chunk, err := r.ReadFrame()
			require.NoError(t, err)
			assert.Equal(t, cbor.FrameTypeChunk, chunk.FrameType)
			assert.Equal(t, reqId.ToString(), chunk.Id.ToString())
			assert.Equal(t, []byte("payload-data"), chunk.Payload)

			// Read STREAM_END
			se, err := r.ReadFrame()
			require.NoError(t, err)
			assert.Equal(t, cbor.FrameTypeStreamEnd, se.FrameType)

			// Read END
			end, err := r.ReadFrame()
			require.NoError(t, err)
			assert.Equal(t, cbor.FrameTypeEnd, end.FrameType)

			// Respond
			w.WriteFrame(cbor.NewEnd(reqId, []byte("ok")))
		})
		pluginReadP.Close()
		pluginWriteP.Close()
	}()

	host := NewPluginHost()
	_, err := host.AttachPlugin(hostReadP, hostWriteP)
	require.NoError(t, err)

	relayRead, engineWrite := net.Pipe()
	engineRead, relayWrite := net.Pipe()

	wg.Add(1)
	go func() {
		defer wg.Done()
		writer := cbor.NewFrameWriter(engineWrite)
		reader := cbor.NewFrameReader(engineRead)

		reqId := cbor.NewMessageIdRandom()
		writer.WriteFrame(cbor.NewReq(reqId, "cap:op=cont", []byte{}, "text/plain"))
		writer.WriteFrame(cbor.NewStreamStart(reqId, "arg-0", "media:bytes"))
		writer.WriteFrame(cbor.NewChunk(reqId, "arg-0", 0, []byte("payload-data")))
		writer.WriteFrame(cbor.NewStreamEnd(reqId, "arg-0"))
		writer.WriteFrame(cbor.NewEnd(reqId, nil))

		// Read response
		frame, err := reader.ReadFrame()
		require.NoError(t, err)
		assert.Equal(t, cbor.FrameTypeEnd, frame.FrameType)
		assert.Equal(t, []byte("ok"), frame.Payload)

		engineWrite.Close()
		engineRead.Close()
	}()

	host.Run(relayRead, relayWrite, nil)
	relayRead.Close()
	relayWrite.Close()
	hostReadP.Close()
	hostWriteP.Close()
	wg.Wait()
}

// TEST419: Plugin HEARTBEAT handled locally (not forwarded to relay)
func TestHeartbeatLocalHandling(t *testing.T) {
	manifest := `{"name":"Test","version":"1.0","caps":[{"urn":"cap:op=hb"}]}`

	hostReadP, pluginWriteP := net.Pipe()
	pluginReadP, hostWriteP := net.Pipe()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		simulatePlugin(t, pluginReadP, pluginWriteP, manifest, func(r *cbor.FrameReader, w *cbor.FrameWriter) {
			// Send heartbeat
			hbId := cbor.NewMessageIdRandom()
			w.WriteFrame(cbor.NewHeartbeat(hbId))

			// Read heartbeat response from host
			resp, err := r.ReadFrame()
			require.NoError(t, err)
			assert.Equal(t, cbor.FrameTypeHeartbeat, resp.FrameType)
			assert.Equal(t, hbId.ToString(), resp.Id.ToString())

			// Now send a LOG to give engine something to read
			logId := cbor.NewMessageIdRandom()
			w.WriteFrame(cbor.NewLog(logId, "info", "heartbeat was answered"))
		})
		pluginReadP.Close()
		pluginWriteP.Close()
	}()

	host := NewPluginHost()
	_, err := host.AttachPlugin(hostReadP, hostWriteP)
	require.NoError(t, err)

	relayRead, engineWrite := net.Pipe()
	engineRead, relayWrite := net.Pipe()

	var receivedTypes []cbor.FrameType

	wg.Add(1)
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(engineRead)
		for {
			frame, err := reader.ReadFrame()
			if err != nil {
				break
			}
			receivedTypes = append(receivedTypes, frame.FrameType)
		}
	}()

	// Let the host run for a short time to process events
	go func() {
		time.Sleep(500 * time.Millisecond)
		engineWrite.Close()
		engineRead.Close()
	}()

	host.Run(relayRead, relayWrite, nil)
	relayRead.Close()
	relayWrite.Close()
	hostReadP.Close()
	hostWriteP.Close()
	wg.Wait()

	// HEARTBEAT must NOT appear in relay
	for _, ft := range receivedTypes {
		assert.NotEqual(t, cbor.FrameTypeHeartbeat, ft, "heartbeat must not be forwarded to relay")
	}
	// LOG must appear (proving the relay did receive forwarded frames)
	found := false
	for _, ft := range receivedTypes {
		if ft == cbor.FrameTypeLog {
			found = true
		}
	}
	assert.True(t, found, "LOG must be forwarded to relay")
}

// TEST420: Plugin non-HELLO/non-HB frames forwarded to relay
func TestPluginFramesForwardedToRelay(t *testing.T) {
	manifest := `{"name":"Test","version":"1.0","caps":[{"urn":"cap:op=fwd"}]}`

	hostReadP, pluginWriteP := net.Pipe()
	pluginReadP, hostWriteP := net.Pipe()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		simulatePlugin(t, pluginReadP, pluginWriteP, manifest, func(r *cbor.FrameReader, w *cbor.FrameWriter) {
			// Read REQ from host
			req, err := r.ReadFrame()
			if err != nil {
				return
			}
			reqId := req.Id

			// Read END
			r.ReadFrame()

			// Send diverse frame types
			w.WriteFrame(cbor.NewLog(reqId, "info", "processing"))
			w.WriteFrame(cbor.NewStreamStart(reqId, "output", "media:bytes"))
			w.WriteFrame(cbor.NewChunk(reqId, "output", 0, []byte("data")))
			w.WriteFrame(cbor.NewStreamEnd(reqId, "output"))
			w.WriteFrame(cbor.NewEnd(reqId, nil))
		})
		pluginReadP.Close()
		pluginWriteP.Close()
	}()

	host := NewPluginHost()
	_, err := host.AttachPlugin(hostReadP, hostWriteP)
	require.NoError(t, err)

	relayRead, engineWrite := net.Pipe()
	engineRead, relayWrite := net.Pipe()

	var receivedTypes []cbor.FrameType

	wg.Add(1)
	go func() {
		defer wg.Done()
		writer := cbor.NewFrameWriter(engineWrite)
		reader := cbor.NewFrameReader(engineRead)

		// Send REQ + END
		reqId := cbor.NewMessageIdRandom()
		writer.WriteFrame(cbor.NewReq(reqId, "cap:op=fwd", []byte{}, "text/plain"))
		writer.WriteFrame(cbor.NewEnd(reqId, nil))

		// Read all forwarded frames
		for {
			frame, err := reader.ReadFrame()
			if err != nil {
				break
			}
			receivedTypes = append(receivedTypes, frame.FrameType)
			if frame.FrameType == cbor.FrameTypeEnd {
				break
			}
		}

		engineWrite.Close()
		engineRead.Close()
	}()

	host.Run(relayRead, relayWrite, nil)
	relayRead.Close()
	relayWrite.Close()
	hostReadP.Close()
	hostWriteP.Close()
	wg.Wait()

	// Verify forwarded types
	typeSet := make(map[cbor.FrameType]bool)
	for _, ft := range receivedTypes {
		typeSet[ft] = true
	}
	assert.True(t, typeSet[cbor.FrameTypeLog], "LOG must be forwarded")
	assert.True(t, typeSet[cbor.FrameTypeStreamStart], "STREAM_START must be forwarded")
	assert.True(t, typeSet[cbor.FrameTypeChunk], "CHUNK must be forwarded")
	assert.True(t, typeSet[cbor.FrameTypeEnd], "END must be forwarded")
}

// TEST421: Plugin death updates capability list (removes dead plugin's caps)
func TestPluginDeathUpdatesCaps(t *testing.T) {
	manifest := `{"name":"Test","version":"1.0","caps":[{"urn":"cap:op=die"}]}`

	hostReadP, pluginWriteP := net.Pipe()
	pluginReadP, hostWriteP := net.Pipe()

	var wg sync.WaitGroup

	// Plugin: handshake then immediately die
	wg.Add(1)
	go func() {
		defer wg.Done()
		simulatePlugin(t, pluginReadP, pluginWriteP, manifest, nil)
		// Die immediately after handshake
		pluginReadP.Close()
		pluginWriteP.Close()
	}()

	host := NewPluginHost()
	_, err := host.AttachPlugin(hostReadP, hostWriteP)
	require.NoError(t, err)

	// Before death: caps must be present
	caps := host.Capabilities()
	assert.NotNil(t, caps)
	assert.Contains(t, string(caps), "cap:op=die")

	relayRead, engineWrite := net.Pipe()
	engineRead, relayWrite := net.Pipe()

	// Let host process the death event briefly
	go func() {
		time.Sleep(500 * time.Millisecond)
		engineWrite.Close()
		engineRead.Close()
	}()

	host.Run(relayRead, relayWrite, nil)

	// After death: caps must be gone
	capsAfter := host.Capabilities()
	if capsAfter != nil {
		var parsed map[string][]string
		json.Unmarshal(capsAfter, &parsed)
		assert.Empty(t, parsed["caps"], "dead plugin caps must be removed")
	}

	relayRead.Close()
	relayWrite.Close()
	hostReadP.Close()
	hostWriteP.Close()
	wg.Wait()
}

// TEST422: Plugin death sends ERR for all pending requests
func TestPluginDeathSendsErr(t *testing.T) {
	manifest := `{"name":"Test","version":"1.0","caps":[{"urn":"cap:op=die"}]}`

	hostReadP, pluginWriteP := net.Pipe()
	pluginReadP, hostWriteP := net.Pipe()

	var wg sync.WaitGroup

	// Plugin: handshake, read REQ, then die
	wg.Add(1)
	go func() {
		defer wg.Done()
		simulatePlugin(t, pluginReadP, pluginWriteP, manifest, func(r *cbor.FrameReader, w *cbor.FrameWriter) {
			// Read REQ
			r.ReadFrame()
			// Die immediately without responding
			pluginReadP.Close()
			pluginWriteP.Close()
		})
	}()

	host := NewPluginHost()
	_, err := host.AttachPlugin(hostReadP, hostWriteP)
	require.NoError(t, err)

	relayRead, engineWrite := net.Pipe()
	engineRead, relayWrite := net.Pipe()

	var errFrame *cbor.Frame

	wg.Add(1)
	go func() {
		defer wg.Done()
		writer := cbor.NewFrameWriter(engineWrite)
		reader := cbor.NewFrameReader(engineRead)

		// Send REQ + END
		reqId := cbor.NewMessageIdRandom()
		writer.WriteFrame(cbor.NewReq(reqId, "cap:op=die", []byte("hello"), "text/plain"))
		writer.WriteFrame(cbor.NewEnd(reqId, nil))

		// Wait for ERR
		for {
			frame, err := reader.ReadFrame()
			if err != nil {
				break
			}
			if frame.FrameType == cbor.FrameTypeErr {
				errFrame = frame
				break
			}
		}

		engineWrite.Close()
		engineRead.Close()
	}()

	host.Run(relayRead, relayWrite, nil)
	relayRead.Close()
	relayWrite.Close()
	hostReadP.Close()
	hostWriteP.Close()
	wg.Wait()

	require.NotNil(t, errFrame, "must receive ERR when plugin dies with pending request")
	assert.Equal(t, "PLUGIN_DIED", errFrame.ErrorCode())
}

// TEST423: Multiple plugins with distinct caps route independently
func TestMultiPluginDistinctCaps(t *testing.T) {
	manifestA := `{"name":"PluginA","version":"1.0","caps":[{"urn":"cap:op=alpha"}]}`
	manifestB := `{"name":"PluginB","version":"1.0","caps":[{"urn":"cap:op=beta"}]}`

	// Plugin A pipes
	hostReadA, pluginWriteA := net.Pipe()
	pluginReadA, hostWriteA := net.Pipe()

	// Plugin B pipes
	hostReadB, pluginWriteB := net.Pipe()
	pluginReadB, hostWriteB := net.Pipe()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		simulatePlugin(t, pluginReadA, pluginWriteA, manifestA, func(r *cbor.FrameReader, w *cbor.FrameWriter) {
			req, err := r.ReadFrame()
			if err != nil {
				return
			}
			// Read until END
			for {
				f, err := r.ReadFrame()
				if err != nil || f.FrameType == cbor.FrameTypeEnd {
					break
				}
			}
			w.WriteFrame(cbor.NewEnd(req.Id, []byte("from-A")))
		})
		pluginReadA.Close()
		pluginWriteA.Close()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		simulatePlugin(t, pluginReadB, pluginWriteB, manifestB, func(r *cbor.FrameReader, w *cbor.FrameWriter) {
			req, err := r.ReadFrame()
			if err != nil {
				return
			}
			for {
				f, err := r.ReadFrame()
				if err != nil || f.FrameType == cbor.FrameTypeEnd {
					break
				}
			}
			w.WriteFrame(cbor.NewEnd(req.Id, []byte("from-B")))
		})
		pluginReadB.Close()
		pluginWriteB.Close()
	}()

	host := NewPluginHost()
	_, err := host.AttachPlugin(hostReadA, hostWriteA)
	require.NoError(t, err)
	_, err = host.AttachPlugin(hostReadB, hostWriteB)
	require.NoError(t, err)

	relayRead, engineWrite := net.Pipe()
	engineRead, relayWrite := net.Pipe()

	responses := make(map[string][]byte)
	var mu sync.Mutex

	wg.Add(1)
	go func() {
		defer wg.Done()
		writer := cbor.NewFrameWriter(engineWrite)
		reader := cbor.NewFrameReader(engineRead)

		// Send REQ for alpha
		alphaId := cbor.NewMessageIdRandom()
		writer.WriteFrame(cbor.NewReq(alphaId, "cap:op=alpha", []byte{}, "text/plain"))
		writer.WriteFrame(cbor.NewEnd(alphaId, nil))

		// Send REQ for beta
		betaId := cbor.NewMessageIdRandom()
		writer.WriteFrame(cbor.NewReq(betaId, "cap:op=beta", []byte{}, "text/plain"))
		writer.WriteFrame(cbor.NewEnd(betaId, nil))

		// Read 2 responses
		for i := 0; i < 2; i++ {
			frame, err := reader.ReadFrame()
			if err != nil {
				break
			}
			if frame.FrameType == cbor.FrameTypeEnd {
				idStr := frame.Id.ToString()
				mu.Lock()
				if idStr == alphaId.ToString() {
					responses["alpha"] = frame.Payload
				} else if idStr == betaId.ToString() {
					responses["beta"] = frame.Payload
				}
				mu.Unlock()
			}
		}

		engineWrite.Close()
		engineRead.Close()
	}()

	host.Run(relayRead, relayWrite, nil)
	relayRead.Close()
	relayWrite.Close()
	hostReadA.Close()
	hostWriteA.Close()
	hostReadB.Close()
	hostWriteB.Close()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []byte("from-A"), responses["alpha"])
	assert.Equal(t, []byte("from-B"), responses["beta"])
}

// TEST424: Concurrent requests to same plugin handled independently
func TestConcurrentRequestsSamePlugin(t *testing.T) {
	manifest := `{"name":"Test","version":"1.0","caps":[{"urn":"cap:op=conc"}]}`

	hostReadP, pluginWriteP := net.Pipe()
	pluginReadP, hostWriteP := net.Pipe()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		simulatePlugin(t, pluginReadP, pluginWriteP, manifest, func(r *cbor.FrameReader, w *cbor.FrameWriter) {
			// Read both REQs and ENDs, respond to each
			var reqIds []cbor.MessageId

			// Read REQ 0
			req0, err := r.ReadFrame()
			if err != nil {
				return
			}
			reqIds = append(reqIds, req0.Id)

			// Read END for req 0
			r.ReadFrame()

			// Read REQ 1
			req1, err := r.ReadFrame()
			if err != nil {
				return
			}
			reqIds = append(reqIds, req1.Id)

			// Read END for req 1
			r.ReadFrame()

			// Respond to each
			w.WriteFrame(cbor.NewEnd(reqIds[0], []byte("response-0")))
			w.WriteFrame(cbor.NewEnd(reqIds[1], []byte("response-1")))
		})
		pluginReadP.Close()
		pluginWriteP.Close()
	}()

	host := NewPluginHost()
	_, err := host.AttachPlugin(hostReadP, hostWriteP)
	require.NoError(t, err)

	relayRead, engineWrite := net.Pipe()
	engineRead, relayWrite := net.Pipe()

	responses := make(map[string][]byte)
	var mu sync.Mutex

	wg.Add(1)
	go func() {
		defer wg.Done()
		writer := cbor.NewFrameWriter(engineWrite)
		reader := cbor.NewFrameReader(engineRead)

		// Send two concurrent REQs
		id0 := cbor.NewMessageIdRandom()
		id1 := cbor.NewMessageIdRandom()

		writer.WriteFrame(cbor.NewReq(id0, "cap:op=conc", []byte{}, "text/plain"))
		writer.WriteFrame(cbor.NewEnd(id0, nil))

		writer.WriteFrame(cbor.NewReq(id1, "cap:op=conc", []byte{}, "text/plain"))
		writer.WriteFrame(cbor.NewEnd(id1, nil))

		// Read both responses
		for i := 0; i < 2; i++ {
			frame, err := reader.ReadFrame()
			if err != nil {
				break
			}
			if frame.FrameType == cbor.FrameTypeEnd {
				idStr := frame.Id.ToString()
				mu.Lock()
				if idStr == id0.ToString() {
					responses["0"] = frame.Payload
				} else if idStr == id1.ToString() {
					responses["1"] = frame.Payload
				}
				mu.Unlock()
			}
		}

		engineWrite.Close()
		engineRead.Close()
	}()

	host.Run(relayRead, relayWrite, nil)
	relayRead.Close()
	relayWrite.Close()
	hostReadP.Close()
	hostWriteP.Close()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []byte("response-0"), responses["0"])
	assert.Equal(t, []byte("response-1"), responses["1"])
}

// TEST425: FindPluginForCap returns false for unknown cap
func TestFindPluginForCapUnknown(t *testing.T) {
	host := NewPluginHost()
	host.RegisterPlugin("/path/to/plugin", []string{"cap:op=known"})

	idx, found := host.FindPluginForCap("cap:op=known")
	assert.True(t, found, "known cap must be found")
	assert.Equal(t, 0, idx)

	_, found = host.FindPluginForCap("cap:op=unknown")
	assert.False(t, found, "unknown cap must not be found")
}
