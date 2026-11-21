package service

import (
	"os"
	"testing"

	"github.com/samautomateai/tessivo/internal/mcp/mcptest"
)

// TestMain lets tests start this binary as a fake MCP server.
func TestMain(m *testing.M) {
	mcptest.Main()
	os.Exit(m.Run())
}
