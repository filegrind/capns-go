package main

import (
	"encoding/json"
	"fmt"
	"os"

	capns "github.com/filegrind/cap-sdk-go"
)

func main() {
	// Create manifest
	manifest := capns.NewCapManifest(
		"testplugin",
		"1.0.0",
		"Test plugin for Go",
		[]capns.Cap{
			{
				Urn:     mustParseCapUrn(`cap:in="media:string;textable;form=scalar";op=echo;out="media:string;textable;form=scalar"`),
				Title:   "Echo",
				Command: "echo",
			},
			{
				Urn:     mustParseCapUrn(`cap:in="media:void";op=void_test;out="media:void"`),
				Title:   "Void Test",
				Command: "void",
			},
		},
	)

	// Create runtime
	runtime, err := capns.NewPluginRuntimeWithManifest(manifest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create runtime: %v\n", err)
		os.Exit(1)
	}

	// Register echo handler
	runtime.Register(`cap:in="media:string;textable;form=scalar";op=echo;out="media:string;textable;form=scalar"`,
		func(payload []byte, emitter capns.StreamEmitter, peer capns.PeerInvoker) error {
			// Parse input JSON
			var input map[string]interface{}
			if err := json.Unmarshal(payload, &input); err != nil {
				return fmt.Errorf("failed to parse input: %w", err)
			}

			// Extract the text field
			text, ok := input["text"].(string)
			if !ok {
				return fmt.Errorf("missing or invalid 'text' field")
			}

			// Echo it back
			response := map[string]string{
				"result": text,
			}

			responseData, err := json.Marshal(response)
			if err != nil {
				return fmt.Errorf("failed to marshal response: %w", err)
			}

			emitter.Emit(responseData)
			return nil
		})

	// Register void test handler
	runtime.Register(`cap:in="media:void";op=void_test;out="media:void"`,
		func(payload []byte, emitter capns.StreamEmitter, peer capns.PeerInvoker) error {
			// Void capability - no input, no output
			emitter.Emit([]byte{})
			return nil
		})

	// Run runtime (auto-detects CLI vs CBOR mode)
	if err := runtime.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
		os.Exit(1)
	}
}

func mustParseCapUrn(urnStr string) *capns.CapUrn {
	urn, err := capns.NewCapUrnFromString(urnStr)
	if err != nil {
		panic(fmt.Sprintf("invalid URN: %s - %v", urnStr, err))
	}
	return urn
}
