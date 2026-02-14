package capns

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/filegrind/capns-go/cbor"
)

// TEST426: Single master REQ/response routing
func TestRelaySwitchSingleMasterReqResponse(t *testing.T) {
	// Create socket pairs
	engineRead, slaveWrite := net.Pipe()
	slaveRead, engineWrite := net.Pipe()

	done := make(chan bool)

	// Spawn mock slave
	go func() {
		reader := cbor.NewFrameReader(slaveRead)
		writer := cbor.NewFrameWriter(slaveWrite)

		// Send initial RelayNotify
		manifest := map[string]interface{}{
			"capabilities": []string{`cap:in=media:;out=media:`},
		}
		manifestJSON, _ := json.Marshal(manifest)
		limits := cbor.DefaultLimits()
		if err := SendNotify(writer, manifestJSON, limits); err != nil {
			t.Errorf("Failed to send notify: %v", err)
			return
		}

		done <- true

		// Read REQ and send response
		frame, err := reader.ReadFrame()
		if err != nil || frame == nil {
			return
		}
		if frame.FrameType == cbor.FrameTypeReq {
			response := cbor.NewEnd(frame.Id, []byte{42})
			writer.WriteFrame(response)
		}
	}()

	<-done

	// Create RelaySwitch
	sw, err := NewRelaySwitch([]SocketPair{{Read: engineRead, Write: engineWrite}})
	if err != nil {
		t.Fatalf("Failed to create RelaySwitch: %v", err)
	}

	// Send REQ
	req := cbor.NewReq(
		cbor.NewMessageIdFromUint(1),
		`cap:in=media:;out=media:`,
		[]byte{1, 2, 3},
		"text/plain",
	)
	if err := sw.SendToMaster(req); err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}

	// Read response
	response, err := sw.ReadFromMasters()
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}
	if response.FrameType != cbor.FrameTypeEnd {
		t.Errorf("Expected END frame, got %d", response.FrameType)
	}
	if response.Id.ToString() != cbor.NewMessageIdFromUint(1).ToString() {
		t.Errorf("ID mismatch")
	}
	if len(response.Payload) != 1 || response.Payload[0] != 42 {
		t.Errorf("Payload mismatch: %v", response.Payload)
	}
}

// TEST427: Multi-master cap routing
func TestRelaySwitchMultiMasterCapRouting(t *testing.T) {
	engineRead1, slaveWrite1 := net.Pipe()
	slaveRead1, engineWrite1 := net.Pipe()
	engineRead2, slaveWrite2 := net.Pipe()
	slaveRead2, engineWrite2 := net.Pipe()

	done1 := make(chan bool)
	done2 := make(chan bool)

	// Spawn slave 1 (echo)
	go func() {
		reader := cbor.NewFrameReader(slaveRead1)
		writer := cbor.NewFrameWriter(slaveWrite1)

		manifest := map[string]interface{}{
			"capabilities": []string{`cap:in=media:;out=media:`},
		}
		manifestJSON, _ := json.Marshal(manifest)
		SendNotify(writer, manifestJSON, cbor.DefaultLimits())
		done1 <- true

		for {
			frame, err := reader.ReadFrame()
			if err != nil || frame == nil {
				return
			}
			if frame.FrameType == cbor.FrameTypeReq {
				response := cbor.NewEnd(frame.Id, []byte{1})
				writer.WriteFrame(response)
			}
		}
	}()

	// Spawn slave 2 (double)
	go func() {
		reader := cbor.NewFrameReader(slaveRead2)
		writer := cbor.NewFrameWriter(slaveWrite2)

		manifest := map[string]interface{}{
			"capabilities": []string{`cap:in="media:void";op=double;out="media:void"`},
		}
		manifestJSON, _ := json.Marshal(manifest)
		SendNotify(writer, manifestJSON, cbor.DefaultLimits())
		done2 <- true

		for {
			frame, err := reader.ReadFrame()
			if err != nil || frame == nil {
				return
			}
			if frame.FrameType == cbor.FrameTypeReq {
				response := cbor.NewEnd(frame.Id, []byte{2})
				writer.WriteFrame(response)
			}
		}
	}()

	<-done1
	<-done2

	sw, err := NewRelaySwitch([]SocketPair{
		{Read: engineRead1, Write: engineWrite1},
		{Read: engineRead2, Write: engineWrite2},
	})
	if err != nil {
		t.Fatalf("Failed to create RelaySwitch: %v", err)
	}

	// Send REQ for echo
	req1 := cbor.NewReq(
		cbor.NewMessageIdFromUint(1),
		`cap:in=media:;out=media:`,
		[]byte{},
		"text/plain",
	)
	sw.SendToMaster(req1)
	resp1, _ := sw.ReadFromMasters()
	if len(resp1.Payload) != 1 || resp1.Payload[0] != 1 {
		t.Errorf("Expected payload [1], got %v", resp1.Payload)
	}

	// Send REQ for double
	req2 := cbor.NewReq(
		cbor.NewMessageIdFromUint(2),
		`cap:in="media:void";op=double;out="media:void"`,
		[]byte{},
		"text/plain",
	)
	sw.SendToMaster(req2)
	resp2, _ := sw.ReadFromMasters()
	if len(resp2.Payload) != 1 || resp2.Payload[0] != 2 {
		t.Errorf("Expected payload [2], got %v", resp2.Payload)
	}
}

// TEST428: Unknown cap returns error
func TestRelaySwitchUnknownCapReturnsError(t *testing.T) {
	engineRead, slaveWrite := net.Pipe()
	slaveRead, engineWrite := net.Pipe()

	done := make(chan bool)

	go func() {
		reader := cbor.NewFrameReader(slaveRead)
		writer := cbor.NewFrameWriter(slaveWrite)

		manifest := map[string]interface{}{
			"capabilities": []string{`cap:in=media:;out=media:`},
		}
		manifestJSON, _ := json.Marshal(manifest)
		SendNotify(writer, manifestJSON, cbor.DefaultLimits())
		done <- true

		// Keep reading to prevent blocking
		for {
			if _, err := reader.ReadFrame(); err != nil {
				return
			}
		}
	}()

	<-done

	sw, err := NewRelaySwitch([]SocketPair{{Read: engineRead, Write: engineWrite}})
	if err != nil {
		t.Fatalf("Failed to create RelaySwitch: %v", err)
	}

	// Send REQ for unknown cap
	req := cbor.NewReq(
		cbor.NewMessageIdFromUint(1),
		`cap:in="media:void";op=unknown;out="media:void"`,
		[]byte{},
		"text/plain",
	)

	err = sw.SendToMaster(req)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if _, ok := err.(*RelaySwitchError); !ok {
		t.Errorf("Expected RelaySwitchError, got %T", err)
	}
}

// TEST429: Cap routing logic (find_master_for_cap)
func TestRelaySwitchFindMasterForCap(t *testing.T) {
	engineRead1, slaveWrite1 := net.Pipe()
	slaveRead1, engineWrite1 := net.Pipe()
	engineRead2, slaveWrite2 := net.Pipe()
	slaveRead2, engineWrite2 := net.Pipe()

	done1 := make(chan bool)
	done2 := make(chan bool)

	go func() {
		reader := cbor.NewFrameReader(slaveRead1)
		writer := cbor.NewFrameWriter(slaveWrite1)
		manifest := map[string]interface{}{
			"capabilities": []string{`cap:in=media:;out=media:`},
		}
		manifestJSON, _ := json.Marshal(manifest)
		SendNotify(writer, manifestJSON, cbor.DefaultLimits())
		done1 <- true
		for {
			if _, err := reader.ReadFrame(); err != nil {
				return
			}
		}
	}()

	go func() {
		reader := cbor.NewFrameReader(slaveRead2)
		writer := cbor.NewFrameWriter(slaveWrite2)
		manifest := map[string]interface{}{
			"capabilities": []string{`cap:in="media:void";op=double;out="media:void"`},
		}
		manifestJSON, _ := json.Marshal(manifest)
		SendNotify(writer, manifestJSON, cbor.DefaultLimits())
		done2 <- true
		for {
			if _, err := reader.ReadFrame(); err != nil {
				return
			}
		}
	}()

	<-done1
	<-done2

	sw, err := NewRelaySwitch([]SocketPair{
		{Read: engineRead1, Write: engineWrite1},
		{Read: engineRead2, Write: engineWrite2},
	})
	if err != nil {
		t.Fatalf("Failed to create RelaySwitch: %v", err)
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()

	// Verify routing
	idx1, err := sw.findMasterForCap(`cap:in=media:;out=media:`)
	if err != nil || idx1 != 0 {
		t.Errorf("Expected master 0 for echo, got %d (err=%v)", idx1, err)
	}

	idx2, err := sw.findMasterForCap(`cap:in="media:void";op=double;out="media:void"`)
	if err != nil || idx2 != 1 {
		t.Errorf("Expected master 1 for double, got %d (err=%v)", idx2, err)
	}

	_, err = sw.findMasterForCap(`cap:in="media:void";op=unknown;out="media:void"`)
	if err == nil {
		t.Error("Expected error for unknown cap")
	}

	// Verify aggregate capabilities
	var caps map[string]interface{}
	json.Unmarshal(sw.capabilities, &caps)
	capList := caps["capabilities"].([]interface{})
	if len(capList) != 2 {
		t.Errorf("Expected 2 capabilities, got %d", len(capList))
	}
}

// TEST430: Tie-breaking (same cap on multiple masters)
func TestRelaySwitchTieBreaking(t *testing.T) {
	engineRead1, slaveWrite1 := net.Pipe()
	slaveRead1, engineWrite1 := net.Pipe()
	engineRead2, slaveWrite2 := net.Pipe()
	slaveRead2, engineWrite2 := net.Pipe()

	done1 := make(chan bool)
	done2 := make(chan bool)

	sameCap := `cap:in=media:;out=media:`

	// Slave 1 responds with [1]
	go func() {
		reader := cbor.NewFrameReader(slaveRead1)
		writer := cbor.NewFrameWriter(slaveWrite1)
		manifest := map[string]interface{}{"capabilities": []string{sameCap}}
		manifestJSON, _ := json.Marshal(manifest)
		SendNotify(writer, manifestJSON, cbor.DefaultLimits())
		done1 <- true

		for {
			frame, err := reader.ReadFrame()
			if err != nil || frame == nil {
				return
			}
			if frame.FrameType == cbor.FrameTypeReq {
				response := cbor.NewEnd(frame.Id, []byte{1})
				writer.WriteFrame(response)
			}
		}
	}()

	// Slave 2 responds with [2]
	go func() {
		reader := cbor.NewFrameReader(slaveRead2)
		writer := cbor.NewFrameWriter(slaveWrite2)
		manifest := map[string]interface{}{"capabilities": []string{sameCap}}
		manifestJSON, _ := json.Marshal(manifest)
		SendNotify(writer, manifestJSON, cbor.DefaultLimits())
		done2 <- true

		for {
			frame, err := reader.ReadFrame()
			if err != nil || frame == nil {
				return
			}
			if frame.FrameType == cbor.FrameTypeReq {
				response := cbor.NewEnd(frame.Id, []byte{2})
				writer.WriteFrame(response)
			}
		}
	}()

	<-done1
	<-done2

	sw, _ := NewRelaySwitch([]SocketPair{
		{Read: engineRead1, Write: engineWrite1},
		{Read: engineRead2, Write: engineWrite2},
	})

	// First request
	req1 := cbor.NewReq(cbor.NewMessageIdFromUint(1), sameCap, []byte{}, "text/plain")
	sw.SendToMaster(req1)
	resp1, _ := sw.ReadFromMasters()
	if len(resp1.Payload) != 1 || resp1.Payload[0] != 1 {
		t.Errorf("First request should route to master 0, got payload %v", resp1.Payload)
	}

	// Second request - should also go to master 0
	req2 := cbor.NewReq(cbor.NewMessageIdFromUint(2), sameCap, []byte{}, "text/plain")
	sw.SendToMaster(req2)
	resp2, _ := sw.ReadFromMasters()
	if len(resp2.Payload) != 1 || resp2.Payload[0] != 1 {
		t.Errorf("Second request should also route to master 0, got payload %v", resp2.Payload)
	}
}

// TEST431: Continuation frame routing
func TestRelaySwitchContinuationFrameRouting(t *testing.T) {
	engineRead, slaveWrite := net.Pipe()
	slaveRead, engineWrite := net.Pipe()

	done := make(chan bool)

	go func() {
		reader := cbor.NewFrameReader(slaveRead)
		writer := cbor.NewFrameWriter(slaveWrite)

		manifest := map[string]interface{}{
			"capabilities": []string{`cap:in="media:void";op=test;out="media:void"`},
		}
		manifestJSON, _ := json.Marshal(manifest)
		SendNotify(writer, manifestJSON, cbor.DefaultLimits())
		done <- true

		// Read REQ
		req, _ := reader.ReadFrame()
		if req.FrameType != cbor.FrameTypeReq {
			t.Errorf("Expected REQ, got %d", req.FrameType)
			return
		}

		// Read CHUNK
		chunk, _ := reader.ReadFrame()
		if chunk.FrameType != cbor.FrameTypeChunk {
			t.Errorf("Expected CHUNK, got %d", chunk.FrameType)
			return
		}
		if chunk.Id.ToString() != req.Id.ToString() {
			t.Error("CHUNK ID mismatch")
			return
		}

		// Read END
		end, _ := reader.ReadFrame()
		if end.FrameType != cbor.FrameTypeEnd {
			t.Errorf("Expected END, got %d", end.FrameType)
			return
		}
		if end.Id.ToString() != req.Id.ToString() {
			t.Error("END ID mismatch")
			return
		}

		// Send response
		response := cbor.NewEnd(req.Id, []byte{42})
		writer.WriteFrame(response)
	}()

	<-done

	sw, _ := NewRelaySwitch([]SocketPair{{Read: engineRead, Write: engineWrite}})

	reqID := cbor.NewMessageIdFromUint(1)

	// Send REQ
	req := cbor.NewReq(reqID, `cap:in="media:void";op=test;out="media:void"`, []byte{}, "text/plain")
	sw.SendToMaster(req)

	// Send CHUNK
	chunk := cbor.NewChunk(reqID, "stream1", 0, []byte{1, 2, 3})
	sw.SendToMaster(chunk)

	// Send END
	end := cbor.NewEnd(reqID, nil)
	sw.SendToMaster(end)

	// Read response
	response, _ := sw.ReadFromMasters()
	if response.FrameType != cbor.FrameTypeEnd {
		t.Errorf("Expected END, got %d", response.FrameType)
	}
	if len(response.Payload) != 1 || response.Payload[0] != 42 {
		t.Errorf("Payload mismatch: %v", response.Payload)
	}
}

// TEST432: Empty masters list returns error
func TestRelaySwitchEmptyMastersListError(t *testing.T) {
	_, err := NewRelaySwitch([]SocketPair{})
	if err == nil {
		t.Fatal("Expected error for empty masters list")
	}
	rsErr, ok := err.(*RelaySwitchError)
	if !ok {
		t.Fatalf("Expected RelaySwitchError, got %T", err)
	}
	if rsErr.Type != RelaySwitchErrorTypeProtocol {
		t.Errorf("Expected Protocol error, got %d", rsErr.Type)
	}
}

// TEST433: Capability aggregation deduplicates
func TestRelaySwitchCapabilityAggregationDeduplicates(t *testing.T) {
	engineRead1, slaveWrite1 := net.Pipe()
	slaveRead1, engineWrite1 := net.Pipe()
	engineRead2, slaveWrite2 := net.Pipe()
	slaveRead2, engineWrite2 := net.Pipe()

	done1 := make(chan bool)
	done2 := make(chan bool)

	go func() {
		reader := cbor.NewFrameReader(slaveRead1)
		writer := cbor.NewFrameWriter(slaveWrite1)
		manifest := map[string]interface{}{
			"capabilities": []string{
				`cap:in=media:;out=media:`,
				`cap:in="media:void";op=double;out="media:void"`,
			},
		}
		manifestJSON, _ := json.Marshal(manifest)
		SendNotify(writer, manifestJSON, cbor.DefaultLimits())
		done1 <- true
		for {
			if _, err := reader.ReadFrame(); err != nil {
				return
			}
		}
	}()

	go func() {
		reader := cbor.NewFrameReader(slaveRead2)
		writer := cbor.NewFrameWriter(slaveWrite2)
		manifest := map[string]interface{}{
			"capabilities": []string{
				`cap:in=media:;out=media:`, // Duplicate
				`cap:in="media:void";op=triple;out="media:void"`,
			},
		}
		manifestJSON, _ := json.Marshal(manifest)
		SendNotify(writer, manifestJSON, cbor.DefaultLimits())
		done2 <- true
		for {
			if _, err := reader.ReadFrame(); err != nil {
				return
			}
		}
	}()

	<-done1
	<-done2

	sw, _ := NewRelaySwitch([]SocketPair{
		{Read: engineRead1, Write: engineWrite1},
		{Read: engineRead2, Write: engineWrite2},
	})

	var caps map[string]interface{}
	json.Unmarshal(sw.Capabilities(), &caps)
	capList := caps["capabilities"].([]interface{})

	// Should have 3 unique caps
	if len(capList) != 3 {
		t.Errorf("Expected 3 unique caps, got %d", len(capList))
	}
}

// TEST434: Limits negotiation takes minimum
func TestRelaySwitchLimitsNegotiationMinimum(t *testing.T) {
	engineRead1, slaveWrite1 := net.Pipe()
	slaveRead1, engineWrite1 := net.Pipe()
	engineRead2, slaveWrite2 := net.Pipe()
	slaveRead2, engineWrite2 := net.Pipe()

	done1 := make(chan bool)
	done2 := make(chan bool)

	go func() {
		reader := cbor.NewFrameReader(slaveRead1)
		writer := cbor.NewFrameWriter(slaveWrite1)
		manifest := map[string]interface{}{"capabilities": []string{}}
		manifestJSON, _ := json.Marshal(manifest)
		limits1 := cbor.Limits{MaxFrame: 1_000_000, MaxChunk: 100_000}
		SendNotify(writer, manifestJSON, limits1)
		done1 <- true
		for {
			if _, err := reader.ReadFrame(); err != nil {
				return
			}
		}
	}()

	go func() {
		reader := cbor.NewFrameReader(slaveRead2)
		writer := cbor.NewFrameWriter(slaveWrite2)
		manifest := map[string]interface{}{"capabilities": []string{}}
		manifestJSON, _ := json.Marshal(manifest)
		limits2 := cbor.Limits{MaxFrame: 2_000_000, MaxChunk: 50_000}
		SendNotify(writer, manifestJSON, limits2)
		done2 <- true
		for {
			if _, err := reader.ReadFrame(); err != nil {
				return
			}
		}
	}()

	<-done1
	<-done2

	sw, _ := NewRelaySwitch([]SocketPair{
		{Read: engineRead1, Write: engineWrite1},
		{Read: engineRead2, Write: engineWrite2},
	})

	limits := sw.Limits()
	if limits.MaxFrame != 1_000_000 {
		t.Errorf("Expected max_frame 1000000, got %d", limits.MaxFrame)
	}
	if limits.MaxChunk != 50_000 {
		t.Errorf("Expected max_chunk 50000, got %d", limits.MaxChunk)
	}
}

// TEST435: URN matching (exact vs accepts())
func TestRelaySwitchURNMatching(t *testing.T) {
	engineRead, slaveWrite := net.Pipe()
	slaveRead, engineWrite := net.Pipe()

	done := make(chan bool)

	registeredCap := `cap:in="media:text;utf8";op=process;out="media:text;utf8"`

	go func() {
		reader := cbor.NewFrameReader(slaveRead)
		writer := cbor.NewFrameWriter(slaveWrite)
		manifest := map[string]interface{}{
			"capabilities": []string{registeredCap},
		}
		manifestJSON, _ := json.Marshal(manifest)
		SendNotify(writer, manifestJSON, cbor.DefaultLimits())
		done <- true

		for {
			frame, err := reader.ReadFrame()
			if err != nil || frame == nil {
				return
			}
			if frame.FrameType == cbor.FrameTypeReq {
				response := cbor.NewEnd(frame.Id, []byte{42})
				writer.WriteFrame(response)
			}
		}
	}()

	<-done
	time.Sleep(10 * time.Millisecond) // Give goroutine time to start reading

	sw, _ := NewRelaySwitch([]SocketPair{{Read: engineRead, Write: engineWrite}})

	// Exact match should work
	req1 := cbor.NewReq(cbor.NewMessageIdFromUint(1), registeredCap, []byte{}, "text/plain")
	if err := sw.SendToMaster(req1); err != nil {
		t.Errorf("Exact match should work: %v", err)
	}
	resp1, _ := sw.ReadFromMasters()
	if len(resp1.Payload) != 1 || resp1.Payload[0] != 42 {
		t.Errorf("Payload mismatch: %v", resp1.Payload)
	}

	// More specific request should NOT match
	req2 := cbor.NewReq(
		cbor.NewMessageIdFromUint(2),
		`cap:in="media:text;utf8;normalized";op=process;out="media:text"`,
		[]byte{},
		"text/plain",
	)
	err := sw.SendToMaster(req2)
	if err == nil {
		t.Error("More specific request should not match less specific registered cap")
	}
	if _, ok := err.(*RelaySwitchError); !ok {
		t.Errorf("Expected RelaySwitchError, got %T", err)
	}
}
