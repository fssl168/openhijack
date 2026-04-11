package hosts

import (
	"io"
	"log"
	"os"
	"testing"
)

func TestRemoveBackupRemovesBackupFile(t *testing.T) {
	hm := NewHostsManager(t.TempDir())
	if err := os.WriteFile(hm.BackupPath(), []byte("backup"), 0600); err != nil {
		t.Fatalf("write backup: %v", err)
	}

	if err := hm.RemoveBackup(log.New(io.Discard, "", 0).Printf); err != nil {
		t.Fatalf("remove backup: %v", err)
	}

	if _, err := os.Stat(hm.BackupPath()); !os.IsNotExist(err) {
		t.Fatalf("expected backup to be removed, stat err = %v", err)
	}
}

func TestRemoveBackupIgnoresMissingFile(t *testing.T) {
	hm := NewHostsManager(t.TempDir())
	if err := hm.RemoveBackup(log.New(io.Discard, "", 0).Printf); err != nil {
		t.Fatalf("remove backup: %v", err)
	}
}
