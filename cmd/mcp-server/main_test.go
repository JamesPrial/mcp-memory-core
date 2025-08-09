// In file: cmd/mcp-server/main_test.go
package main

import (
	"bufio"
	"encoding/json"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStdioIntegration(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "test-server")
	err := cmd.Run()
	require.NoError(t, err, "Failed to build server binary")

	serverCmd := exec.Command("./test-server")
	stdin, err := serverCmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := serverCmd.StdoutPipe()
	require.NoError(t, err)

	err = serverCmd.Start()
	require.NoError(t, err)
	defer serverCmd.Process.Kill()

	reader := bufio.NewReader(stdout)

	listReq := map[string]interface{} {
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}
	reqBytes, _ := json.Marshal(listReq)
	stdin.Write(append(reqBytes, '\n'))

	line, err := reader.ReadBytes('\n')
	require.NoError(t, err)

	var listResp map[string]interface{}
	err = json.Unmarshal(line, &listResp)
	require.NoError(t, err)

	assert.Equal(t, float64(1), listResp["id"])
	assert.NotNil(t, listResp["result"])
}