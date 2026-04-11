package cert

import (
	"io"
	"log"
	"os"
	"testing"
)

func TestRemoveLocalArtifactsRemovesGeneratedFiles(t *testing.T) {
	dataDir := t.TempDir()
	certMgr := NewCertManager(dataDir)

	if err := os.MkdirAll(certMgr.CADir(), 0755); err != nil {
		t.Fatalf("mkdir ca dir: %v", err)
	}

	paths := []string{
		certMgr.CACertFile(),
		certMgr.CAKeyFile(),
		certMgr.SrvCertFile(),
		certMgr.SrvKeyFile(),
	}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("test"), 0600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	if err := certMgr.RemoveLocalArtifacts(log.New(io.Discard, "", 0).Printf); err != nil {
		t.Fatalf("remove local artifacts: %v", err)
	}

	for _, path := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, stat err = %v", path, err)
		}
	}
	if _, err := os.Stat(certMgr.CADir()); !os.IsNotExist(err) {
		t.Fatalf("expected CA dir to be removed, stat err = %v", err)
	}
}
