package scraper

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDescribeDownloadDir(t *testing.T) {
	t.Run("missing directory", func(t *testing.T) {
		got := describeDownloadDir(filepath.Join(t.TempDir(), "does-not-exist"), time.Now())
		if !strings.Contains(got, "unreadable") {
			t.Errorf("missing dir: got %q, want it to mention 'unreadable'", got)
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		got := describeDownloadDir(t.TempDir(), time.Now())
		if !strings.Contains(got, "is empty") {
			t.Errorf("empty dir: got %q, want it to mention 'is empty'", got)
		}
	})

	t.Run("flags new and partial entries", func(t *testing.T) {
		dir := t.TempDir()
		startTime := time.Now()

		// write creates a file with the given content and modification time.
		write := func(name, content string, modTime time.Time) {
			p := filepath.Join(dir, name)
			if err := os.WriteFile(p, []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(p, modTime, modTime); err != nil {
				t.Fatal(err)
			}
		}

		// A completed file from before the download started.
		write("old.csv", "old", startTime.Add(-time.Hour))
		// A file produced at/after the download start.
		write("new.csv", "brand new", startTime.Add(time.Second))
		// A Chrome partial-download file.
		write("pending.csv.crdownload", "partial", startTime.Add(time.Second))

		got := describeDownloadDir(dir, startTime)

		// old.csv predates startTime: size only, no "new".
		if !strings.Contains(got, "old.csv(3B)") {
			t.Errorf("old.csv want 'old.csv(3B)' (no new flag): %q", got)
		}
		// new.csv is at/after startTime: flagged "new".
		if !strings.Contains(got, "new.csv(9B,new)") {
			t.Errorf("new.csv want 'new.csv(9B,new)': %q", got)
		}
		// .crdownload is both new and a partial download.
		if !strings.Contains(got, "pending.csv.crdownload(7B,new,partial)") {
			t.Errorf("crdownload want 'pending.csv.crdownload(7B,new,partial)': %q", got)
		}
	})
}
