package main

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

// TestDatabaseFailureDoesNotLogCredentials exercises the fatal startup path with a synthetic secret.
func TestDatabaseFailureDoesNotLogCredentials(t *testing.T) {
	if os.Getenv("TEST_DATABASE_FAILURE_CHILD") == "1" {
		main()
		t.Fatal("server unexpectedly started")
	}

	const secret = "synthetic-db-password"
	command := exec.Command(os.Args[0], "-test.run=^TestDatabaseFailureDoesNotLogCredentials$")
	command.Env = append(os.Environ(),
		"TEST_DATABASE_FAILURE_CHILD=1",
		"DATABASE_URL=postgres://test:"+secret+"%not-escaped@127.0.0.1:1/test",
		"JWT_SECRET=synthetic-test-secret-at-least-thirty-two-characters",
	)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("startup should fail for a malformed database URL")
	}
	if !bytes.Contains(output, []byte("database connection failed")) {
		t.Fatalf("missing useful failure message: %s", output)
	}
	if bytes.Contains(output, []byte(secret)) || bytes.Contains(output, []byte("DATABASE_URL=")) {
		t.Fatal("database credentials appeared in failure output")
	}
}
