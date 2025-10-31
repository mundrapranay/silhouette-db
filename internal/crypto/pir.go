package crypto

// PIRServer defines the server-side interface for a Private Information Retrieval scheme.
type PIRServer interface {
	// ProcessQuery takes the database (the OKVS blob) and an opaque
	// client query, and returns an opaque server response.
	// The server learns nothing about the item being queried.
	ProcessQuery(db []byte, query []byte) ([]byte, error)
}

// PIRClient defines the client-side interface for generating PIR queries.
// This is typically used by workers to generate queries before calling GetValue.
type PIRClient interface {
	// GenerateQuery creates an opaque PIR query for a specific key.
	// The key is only known to the client, not the server.
	GenerateQuery(key string) ([]byte, error)

	// DecodeResponse decodes a PIR server response to extract the value.
	DecodeResponse(response []byte) ([]byte, error)
}

// TODO: Implement FrodoPIR wrapper using cgo
// This will wrap the Rust implementation of FrodoPIR from:
// https://github.com/brave-experiments/frodo-pir
//
// The implementation should:
// 1. Create a C-compatible FFI wrapper in Rust around FrodoPIR functions
// 2. Compile the Rust code as a static library with C-compatible symbols
// 3. Use cgo to call the C functions from Go
// 4. Handle memory management properly (CGO memory semantics)
//
// Steps:
// - Add frodo-pir as a git submodule
// - Create a Rust FFI wrapper (e.g., in frodo-pir-ffi/)
// - Build static library (.a file)
// - Link it in the Go code using #cgo directives

// MockPIRServer is a placeholder implementation for testing.
// This should be replaced with the actual FrodoPIR wrapper.
type MockPIRServer struct{}

// ProcessQuery implements PIRServer with a mock implementation.
// This is a placeholder - the real implementation will use FrodoPIR.
func (m *MockPIRServer) ProcessQuery(db []byte, query []byte) ([]byte, error) {
	// TODO: Replace with actual FrodoPIR processing
	// For now, just return the query back (not actually private)
	return query, nil
}

// MockPIRClient is a placeholder implementation for testing.
type MockPIRClient struct{}

// GenerateQuery implements PIRClient with a mock implementation.
func (m *MockPIRClient) GenerateQuery(key string) ([]byte, error) {
	// TODO: Replace with actual FrodoPIR query generation
	return []byte("query:" + key), nil
}

// DecodeResponse implements PIRClient with a mock implementation.
func (m *MockPIRClient) DecodeResponse(response []byte) ([]byte, error) {
	// TODO: Replace with actual FrodoPIR response decoding
	return response, nil
}
