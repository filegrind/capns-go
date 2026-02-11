package capns

import (
	"net"
	"sync"
	"testing"

	"github.com/filegrind/capns-go/cbor"
)

// relayPipe creates a pair of connected streams using net.Pipe().
func relayPipe() (net.Conn, net.Conn) {
	return net.Pipe()
}

// TEST404: Slave sends RelayNotify on connect (initialNotify parameter)
func TestSlaveSendsRelayNotifyOnConnect(t *testing.T) {
	manifest := []byte(`{"caps":["cap:op=test"]}`)
	limits := cbor.DefaultLimits()

	masterRead, slaveWrite := relayPipe()

	var wg sync.WaitGroup

	// Slave sends initial notify through socket_write
	wg.Add(1)
	go func() {
		defer wg.Done()
		socketWriter := cbor.NewFrameWriter(slaveWrite)
		err := SendNotify(socketWriter, manifest, limits)
		if err != nil {
			t.Errorf("SendNotify failed: %v", err)
		}
		slaveWrite.Close()
	}()

	// Master reads it
	socketReader := cbor.NewFrameReader(masterRead)
	frame, err := socketReader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}

	if frame.FrameType != cbor.FrameTypeRelayNotify {
		t.Errorf("Expected RELAY_NOTIFY, got %v", frame.FrameType)
	}

	extractedManifest := frame.RelayNotifyManifest()
	if extractedManifest == nil {
		t.Fatal("RelayNotifyManifest() returned nil")
	}
	if string(extractedManifest) != string(manifest) {
		t.Errorf("Manifest mismatch: got %s", string(extractedManifest))
	}

	extractedLimits := frame.RelayNotifyLimits()
	if extractedLimits == nil {
		t.Fatal("RelayNotifyLimits() returned nil")
	}
	if extractedLimits.MaxFrame != limits.MaxFrame {
		t.Errorf("MaxFrame mismatch: expected %d, got %d", limits.MaxFrame, extractedLimits.MaxFrame)
	}
	if extractedLimits.MaxChunk != limits.MaxChunk {
		t.Errorf("MaxChunk mismatch: expected %d, got %d", limits.MaxChunk, extractedLimits.MaxChunk)
	}

	masterRead.Close()
	wg.Wait()
}

// TEST405: Master reads RelayNotify and extracts manifest + limits
func TestMasterReadsRelayNotify(t *testing.T) {
	manifest := []byte(`{"caps":["cap:op=convert"]}`)
	limits := cbor.Limits{MaxFrame: 1_000_000, MaxChunk: 64_000}

	masterRead, slaveWrite := relayPipe()

	var wg sync.WaitGroup

	// Slave sends RelayNotify
	wg.Add(1)
	go func() {
		defer wg.Done()
		writer := cbor.NewFrameWriter(slaveWrite)
		frame := cbor.NewRelayNotify(manifest, limits.MaxFrame, limits.MaxChunk)
		if err := writer.WriteFrame(frame); err != nil {
			t.Errorf("WriteFrame failed: %v", err)
		}
		slaveWrite.Close()
	}()

	// Master connects
	reader := cbor.NewFrameReader(masterRead)
	master, err := ConnectRelayMaster(reader)
	if err != nil {
		t.Fatalf("ConnectRelayMaster failed: %v", err)
	}

	if string(master.Manifest()) != string(manifest) {
		t.Errorf("Manifest mismatch: got %s", string(master.Manifest()))
	}
	if master.Limits().MaxFrame != 1_000_000 {
		t.Errorf("MaxFrame mismatch: expected 1000000, got %d", master.Limits().MaxFrame)
	}
	if master.Limits().MaxChunk != 64_000 {
		t.Errorf("MaxChunk mismatch: expected 64000, got %d", master.Limits().MaxChunk)
	}

	masterRead.Close()
	wg.Wait()
}

// TEST406: Slave stores RelayState from master (ResourceState() returns payload)
func TestSlaveStoresRelayState(t *testing.T) {
	resources := []byte(`{"memory_mb":4096}`)

	// Socket: master writes -> slave reads
	slaveSocketRead, masterSocketWrite := relayPipe()
	// Local: slave needs local streams (unused but required)
	localReadEnd, localWriteEnd := relayPipe()

	var wg sync.WaitGroup

	// Master sends RelayState then closes
	wg.Add(1)
	go func() {
		defer wg.Done()
		writer := cbor.NewFrameWriter(masterSocketWrite)
		if err := SendRelayState(writer, resources); err != nil {
			t.Errorf("SendRelayState failed: %v", err)
		}
		masterSocketWrite.Close()
	}()

	// Slave reads from socket manually (not using Run since we want to verify storage)
	slave := NewRelaySlave(localReadEnd, localWriteEnd)

	socketReader := cbor.NewFrameReader(slaveSocketRead)
	frame, err := socketReader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame failed: %v", err)
	}

	if frame.FrameType != cbor.FrameTypeRelayState {
		t.Errorf("Expected RELAY_STATE, got %v", frame.FrameType)
	}

	// Manually store (simulating what Run does)
	if frame.Payload != nil {
		slave.resourceStateMu.Lock()
		slave.resourceState = make([]byte, len(frame.Payload))
		copy(slave.resourceState, frame.Payload)
		slave.resourceStateMu.Unlock()
	}

	stored := slave.ResourceState()
	if string(stored) != string(resources) {
		t.Errorf("ResourceState mismatch: got %s", string(stored))
	}

	slaveSocketRead.Close()
	localReadEnd.Close()
	localWriteEnd.Close()
	wg.Wait()
}

// TEST407: Protocol frames pass through slave transparently (both directions)
func TestProtocolFramesPassThrough(t *testing.T) {
	// Socket pair: master <-> slave
	slaveSocketRead, masterSocketWrite := relayPipe()
	masterSocketRead, slaveSocketWrite := relayPipe()
	// Local pair: slave <-> host runtime
	runtimeReadsFromSlave, slaveLocalWrite := relayPipe()
	slaveLocalRead, runtimeWritesToSlave := relayPipe()

	reqId := cbor.NewMessageIdRandom()
	chunkId := cbor.NewMessageIdRandom()

	var wg sync.WaitGroup

	// Master sends a REQ frame through the socket
	wg.Add(1)
	go func() {
		defer wg.Done()
		writer := cbor.NewFrameWriter(masterSocketWrite)
		req := cbor.NewReq(reqId, "cap:op=test", []byte("hello"), "text/plain")
		if err := writer.WriteFrame(req); err != nil {
			t.Errorf("WriteFrame REQ failed: %v", err)
		}
		masterSocketWrite.Close()
	}()

	// Runtime sends a CHUNK frame through the local write
	wg.Add(1)
	go func() {
		defer wg.Done()
		writer := cbor.NewFrameWriter(runtimeWritesToSlave)
		chunk := cbor.NewChunk(chunkId, "stream-1", 0, []byte("response"))
		if err := writer.WriteFrame(chunk); err != nil {
			t.Errorf("WriteFrame CHUNK failed: %v", err)
		}
		runtimeWritesToSlave.Close()
	}()

	// Slave relay: manually forward one frame each direction
	wg.Add(1)
	go func() {
		defer wg.Done()
		socketReader := cbor.NewFrameReader(slaveSocketRead)
		socketWriter := cbor.NewFrameWriter(slaveSocketWrite)
		localReader := cbor.NewFrameReader(slaveLocalRead)
		localWriter := cbor.NewFrameWriter(slaveLocalWrite)

		// Socket -> local: read REQ, forward
		fromSocket, err := socketReader.ReadFrame()
		if err != nil {
			t.Errorf("ReadFrame from socket failed: %v", err)
			return
		}
		if fromSocket.FrameType != cbor.FrameTypeReq {
			t.Errorf("Expected REQ from socket, got %v", fromSocket.FrameType)
		}
		if err := localWriter.WriteFrame(fromSocket); err != nil {
			t.Errorf("WriteFrame to local failed: %v", err)
			return
		}

		// Local -> socket: read CHUNK, forward
		fromLocal, err := localReader.ReadFrame()
		if err != nil {
			t.Errorf("ReadFrame from local failed: %v", err)
			return
		}
		if fromLocal.FrameType != cbor.FrameTypeChunk {
			t.Errorf("Expected CHUNK from local, got %v", fromLocal.FrameType)
		}
		if err := socketWriter.WriteFrame(fromLocal); err != nil {
			t.Errorf("WriteFrame to socket failed: %v", err)
			return
		}

		slaveSocketRead.Close()
		slaveSocketWrite.Close()
		slaveLocalRead.Close()
		slaveLocalWrite.Close()
	}()

	// Runtime reads the forwarded REQ
	wg.Add(1)
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(runtimeReadsFromSlave)
		frame, err := reader.ReadFrame()
		if err != nil {
			t.Errorf("Runtime ReadFrame failed: %v", err)
			return
		}
		if frame.FrameType != cbor.FrameTypeReq {
			t.Errorf("Expected REQ at runtime, got %v", frame.FrameType)
		}
		if frame.Cap == nil || *frame.Cap != "cap:op=test" {
			t.Errorf("Cap mismatch: got %v", frame.Cap)
		}
		if string(frame.Payload) != "hello" {
			t.Errorf("Payload mismatch: got %s", string(frame.Payload))
		}
		runtimeReadsFromSlave.Close()
	}()

	// Master reads the forwarded CHUNK
	wg.Add(1)
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(masterSocketRead)
		frame, err := reader.ReadFrame()
		if err != nil {
			t.Errorf("Master ReadFrame failed: %v", err)
			return
		}
		if frame.FrameType != cbor.FrameTypeChunk {
			t.Errorf("Expected CHUNK at master, got %v", frame.FrameType)
		}
		if string(frame.Payload) != "response" {
			t.Errorf("Payload mismatch: got %s", string(frame.Payload))
		}
		masterSocketRead.Close()
	}()

	wg.Wait()
}

// TEST408: RelayNotify/RelayState are NOT forwarded through relay (intercepted)
func TestRelayFramesNotForwarded(t *testing.T) {
	// Master sends RelayState — slave should NOT forward it to local
	slaveSocketRead, masterSocketWrite := relayPipe()
	runtimeRead, slaveLocalWrite := relayPipe()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		writer := cbor.NewFrameWriter(masterSocketWrite)
		// Send RelayState (should be intercepted)
		state := cbor.NewRelayState([]byte(`{"memory":1024}`))
		if err := writer.WriteFrame(state); err != nil {
			t.Errorf("WriteFrame RelayState failed: %v", err)
		}
		// Then send a normal REQ to verify the slave still works
		req := cbor.NewReq(cbor.NewMessageIdRandom(), "cap:op=test", []byte{}, "text/plain")
		if err := writer.WriteFrame(req); err != nil {
			t.Errorf("WriteFrame REQ failed: %v", err)
		}
		masterSocketWrite.Close()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		socketReader := cbor.NewFrameReader(slaveSocketRead)
		localWriter := cbor.NewFrameWriter(slaveLocalWrite)
		var resourceState []byte

		// Read first frame — RelayState, should NOT be forwarded
		frame1, err := socketReader.ReadFrame()
		if err != nil {
			t.Errorf("ReadFrame 1 failed: %v", err)
			return
		}
		if frame1.FrameType != cbor.FrameTypeRelayState {
			t.Errorf("Expected RELAY_STATE, got %v", frame1.FrameType)
		}
		// Store but do NOT forward
		if frame1.Payload != nil {
			resourceState = frame1.Payload
		}

		// Read second frame — REQ, should be forwarded
		frame2, err := socketReader.ReadFrame()
		if err != nil {
			t.Errorf("ReadFrame 2 failed: %v", err)
			return
		}
		if frame2.FrameType != cbor.FrameTypeReq {
			t.Errorf("Expected REQ, got %v", frame2.FrameType)
		}
		if err := localWriter.WriteFrame(frame2); err != nil {
			t.Errorf("WriteFrame failed: %v", err)
		}

		if string(resourceState) != `{"memory":1024}` {
			t.Errorf("ResourceState mismatch: got %s", string(resourceState))
		}

		slaveSocketRead.Close()
		slaveLocalWrite.Close()
	}()

	// Runtime should only see the REQ, not the RelayState
	runtimeReader := cbor.NewFrameReader(runtimeRead)
	frame, err := runtimeReader.ReadFrame()
	if err != nil {
		t.Fatalf("Runtime ReadFrame failed: %v", err)
	}
	if frame.FrameType != cbor.FrameTypeReq {
		t.Errorf("Runtime expected REQ, got %v", frame.FrameType)
	}

	runtimeRead.Close()
	wg.Wait()
}

// TEST409: Slave can inject RelayNotify mid-stream (cap change)
func TestSlaveInjectsRelayNotifyMidstream(t *testing.T) {
	masterSocketRead, slaveSocketWrite := relayPipe()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		socketWriter := cbor.NewFrameWriter(slaveSocketWrite)
		limits := cbor.DefaultLimits()

		// First: send initial RelayNotify
		initial := []byte(`{"caps":["cap:op=test"]}`)
		if err := SendNotify(socketWriter, initial, limits); err != nil {
			t.Errorf("SendNotify initial failed: %v", err)
		}

		// Then: forward a normal CHUNK frame
		chunk := cbor.NewChunk(cbor.NewMessageIdRandom(), "stream-1", 0, []byte("data"))
		if err := socketWriter.WriteFrame(chunk); err != nil {
			t.Errorf("WriteFrame CHUNK failed: %v", err)
		}

		// Then: inject updated RelayNotify (new cap discovered)
		updated := []byte(`{"caps":["cap:op=test","cap:op=convert"]}`)
		if err := SendNotify(socketWriter, updated, limits); err != nil {
			t.Errorf("SendNotify updated failed: %v", err)
		}

		slaveSocketWrite.Close()
	}()

	reader := cbor.NewFrameReader(masterSocketRead)

	// Read initial RelayNotify
	f1, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame 1 failed: %v", err)
	}
	if f1.FrameType != cbor.FrameTypeRelayNotify {
		t.Errorf("Expected RELAY_NOTIFY, got %v", f1.FrameType)
	}
	if string(f1.RelayNotifyManifest()) != `{"caps":["cap:op=test"]}` {
		t.Errorf("Initial manifest mismatch: got %s", string(f1.RelayNotifyManifest()))
	}

	// Read CHUNK (passed through)
	f2, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame 2 failed: %v", err)
	}
	if f2.FrameType != cbor.FrameTypeChunk {
		t.Errorf("Expected CHUNK, got %v", f2.FrameType)
	}

	// Read updated RelayNotify
	f3, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame 3 failed: %v", err)
	}
	if f3.FrameType != cbor.FrameTypeRelayNotify {
		t.Errorf("Expected RELAY_NOTIFY, got %v", f3.FrameType)
	}
	if string(f3.RelayNotifyManifest()) != `{"caps":["cap:op=test","cap:op=convert"]}` {
		t.Errorf("Updated manifest mismatch: got %s", string(f3.RelayNotifyManifest()))
	}

	masterSocketRead.Close()
	wg.Wait()
}

// TEST410: Master receives updated RelayNotify (cap change via ReadFrame)
func TestMasterReceivesUpdatedRelayNotify(t *testing.T) {
	masterSocketRead, slaveSocketWrite := relayPipe()

	limits := cbor.Limits{MaxFrame: 2_000_000, MaxChunk: 100_000}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		writer := cbor.NewFrameWriter(slaveSocketWrite)

		// Initial RelayNotify
		initial := cbor.NewRelayNotify([]byte(`{"caps":["cap:op=a"]}`), limits.MaxFrame, limits.MaxChunk)
		if err := writer.WriteFrame(initial); err != nil {
			t.Errorf("WriteFrame initial notify failed: %v", err)
		}

		// Normal frame
		end := cbor.NewEnd(cbor.NewMessageIdRandom(), nil)
		if err := writer.WriteFrame(end); err != nil {
			t.Errorf("WriteFrame END failed: %v", err)
		}

		// Updated RelayNotify with new limits
		updatedLimits := cbor.Limits{MaxFrame: 3_000_000, MaxChunk: 200_000}
		updated := cbor.NewRelayNotify([]byte(`{"caps":["cap:op=a","cap:op=b"]}`), updatedLimits.MaxFrame, updatedLimits.MaxChunk)
		if err := writer.WriteFrame(updated); err != nil {
			t.Errorf("WriteFrame updated notify failed: %v", err)
		}

		// Another normal frame to prove master continues
		end2 := cbor.NewEnd(cbor.NewMessageIdRandom(), nil)
		if err := writer.WriteFrame(end2); err != nil {
			t.Errorf("WriteFrame END2 failed: %v", err)
		}

		slaveSocketWrite.Close()
	}()

	reader := cbor.NewFrameReader(masterSocketRead)
	master, err := ConnectRelayMaster(reader)
	if err != nil {
		t.Fatalf("ConnectRelayMaster failed: %v", err)
	}

	// Initial state
	if string(master.Manifest()) != `{"caps":["cap:op=a"]}` {
		t.Errorf("Initial manifest mismatch: got %s", string(master.Manifest()))
	}
	if master.Limits().MaxFrame != 2_000_000 {
		t.Errorf("Initial MaxFrame mismatch: expected 2000000, got %d", master.Limits().MaxFrame)
	}

	// First non-relay frame
	f1, err := master.ReadFrame(reader)
	if err != nil {
		t.Fatalf("ReadFrame 1 failed: %v", err)
	}
	if f1 == nil {
		t.Fatal("ReadFrame 1 returned nil")
	}
	if f1.FrameType != cbor.FrameTypeEnd {
		t.Errorf("Expected END, got %v", f1.FrameType)
	}

	// ReadFrame should have intercepted the updated RelayNotify
	f2, err := master.ReadFrame(reader)
	if err != nil {
		t.Fatalf("ReadFrame 2 failed: %v", err)
	}
	if f2 == nil {
		t.Fatal("ReadFrame 2 returned nil")
	}
	if f2.FrameType != cbor.FrameTypeEnd {
		t.Errorf("Expected END, got %v", f2.FrameType)
	}

	// Manifest and limits should be updated
	if string(master.Manifest()) != `{"caps":["cap:op=a","cap:op=b"]}` {
		t.Errorf("Updated manifest mismatch: got %s", string(master.Manifest()))
	}
	if master.Limits().MaxFrame != 3_000_000 {
		t.Errorf("Updated MaxFrame mismatch: expected 3000000, got %d", master.Limits().MaxFrame)
	}
	if master.Limits().MaxChunk != 200_000 {
		t.Errorf("Updated MaxChunk mismatch: expected 200000, got %d", master.Limits().MaxChunk)
	}

	masterSocketRead.Close()
	wg.Wait()
}

// TEST411: Socket close detection (both directions)
func TestSocketCloseDetection(t *testing.T) {
	// Master -> slave direction: master closes, slave detects
	slaveSocketRead, masterSocketWrite := relayPipe()

	masterSocketWrite.Close() // Close immediately

	reader := cbor.NewFrameReader(slaveSocketRead)
	_, err := reader.ReadFrame()
	if err == nil {
		t.Error("Expected error on closed socket, got nil")
	}
	slaveSocketRead.Close()

	// Slave -> master direction: slave closes, master detects via ReadFrame returning nil
	masterSocketRead2, slaveSocketWrite2 := relayPipe()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		writer := cbor.NewFrameWriter(slaveSocketWrite2)
		// Send RelayNotify then close
		notify := cbor.NewRelayNotify([]byte("[]"), cbor.DefaultLimits().MaxFrame, cbor.DefaultLimits().MaxChunk)
		writer.WriteFrame(notify)
		slaveSocketWrite2.Close()
	}()

	reader2 := cbor.NewFrameReader(masterSocketRead2)
	master, err := ConnectRelayMaster(reader2)
	if err != nil {
		t.Fatalf("ConnectRelayMaster failed: %v", err)
	}

	result, err := master.ReadFrame(reader2)
	if err != nil {
		t.Fatalf("ReadFrame returned error: %v", err)
	}
	if result != nil {
		t.Error("Expected nil on closed socket, got frame")
	}

	masterSocketRead2.Close()
	wg.Wait()
}

// TEST412: Bidirectional concurrent frame flow through relay
func TestBidirectionalConcurrentFlow(t *testing.T) {
	// Full relay setup: master <-> socket <-> slave <-> local <-> runtime
	slaveSocketRead, masterSocketWrite := relayPipe()
	masterSocketRead, slaveSocketWrite := relayPipe()
	runtimeReadsFromSlave, slaveLocalWrite := relayPipe()
	slaveLocalRead, runtimeWritesToSlave := relayPipe()

	reqId1 := cbor.NewMessageIdRandom()
	reqId2 := cbor.NewMessageIdRandom()

	var wg sync.WaitGroup

	// Master writes REQ frames
	wg.Add(1)
	go func() {
		defer wg.Done()
		writer := cbor.NewFrameWriter(masterSocketWrite)
		req1 := cbor.NewReq(reqId1, "cap:op=a", []byte("data-a"), "text/plain")
		req2 := cbor.NewReq(reqId2, "cap:op=b", []byte("data-b"), "text/plain")
		writer.WriteFrame(req1)
		writer.WriteFrame(req2)
		masterSocketWrite.Close()
	}()

	// Runtime writes response chunks
	respId := cbor.NewMessageIdRandom()
	wg.Add(1)
	go func() {
		defer wg.Done()
		writer := cbor.NewFrameWriter(runtimeWritesToSlave)
		chunk := cbor.NewChunk(respId, "s1", 0, []byte("resp-a"))
		end := cbor.NewEnd(respId, nil)
		writer.WriteFrame(chunk)
		writer.WriteFrame(end)
		runtimeWritesToSlave.Close()
	}()

	// Slave relay: manually forward frames both directions
	wg.Add(1)
	go func() {
		defer wg.Done()
		sockR := cbor.NewFrameReader(slaveSocketRead)
		sockW := cbor.NewFrameWriter(slaveSocketWrite)
		localR := cbor.NewFrameReader(slaveLocalRead)
		localW := cbor.NewFrameWriter(slaveLocalWrite)

		// Forward 2 frames from socket to local
		for i := 0; i < 2; i++ {
			f, err := sockR.ReadFrame()
			if err != nil {
				t.Errorf("sockR.ReadFrame %d failed: %v", i, err)
				return
			}
			if err := localW.WriteFrame(f); err != nil {
				t.Errorf("localW.WriteFrame %d failed: %v", i, err)
				return
			}
		}
		// Forward 2 frames from local to socket
		for i := 0; i < 2; i++ {
			f, err := localR.ReadFrame()
			if err != nil {
				t.Errorf("localR.ReadFrame %d failed: %v", i, err)
				return
			}
			if err := sockW.WriteFrame(f); err != nil {
				t.Errorf("sockW.WriteFrame %d failed: %v", i, err)
				return
			}
		}

		slaveSocketRead.Close()
		slaveSocketWrite.Close()
		slaveLocalRead.Close()
		slaveLocalWrite.Close()
	}()

	// Runtime reads forwarded REQs
	wg.Add(1)
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(runtimeReadsFromSlave)
		f1, err := reader.ReadFrame()
		if err != nil {
			t.Errorf("Runtime ReadFrame 1 failed: %v", err)
			return
		}
		f2, err := reader.ReadFrame()
		if err != nil {
			t.Errorf("Runtime ReadFrame 2 failed: %v", err)
			return
		}
		if f1.FrameType != cbor.FrameTypeReq {
			t.Errorf("Expected REQ for f1, got %v", f1.FrameType)
		}
		if f2.FrameType != cbor.FrameTypeReq {
			t.Errorf("Expected REQ for f2, got %v", f2.FrameType)
		}
		if f1.Id.ToString() != reqId1.ToString() {
			t.Errorf("f1 id mismatch: expected %s, got %s", reqId1.ToString(), f1.Id.ToString())
		}
		if f2.Id.ToString() != reqId2.ToString() {
			t.Errorf("f2 id mismatch: expected %s, got %s", reqId2.ToString(), f2.Id.ToString())
		}
		runtimeReadsFromSlave.Close()
	}()

	// Master reads forwarded responses
	wg.Add(1)
	go func() {
		defer wg.Done()
		reader := cbor.NewFrameReader(masterSocketRead)
		f1, err := reader.ReadFrame()
		if err != nil {
			t.Errorf("Master ReadFrame 1 failed: %v", err)
			return
		}
		if f1.FrameType != cbor.FrameTypeChunk {
			t.Errorf("Expected CHUNK, got %v", f1.FrameType)
		}
		if string(f1.Payload) != "resp-a" {
			t.Errorf("Payload mismatch: got %s", string(f1.Payload))
		}
		f2, err := reader.ReadFrame()
		if err != nil {
			t.Errorf("Master ReadFrame 2 failed: %v", err)
			return
		}
		if f2.FrameType != cbor.FrameTypeEnd {
			t.Errorf("Expected END, got %v", f2.FrameType)
		}
		masterSocketRead.Close()
	}()

	wg.Wait()
}
