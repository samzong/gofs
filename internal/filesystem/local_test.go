package filesystem

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func newContainmentFixture(t *testing.T) (*Local, string) {
	t.Helper()
	base := t.TempDir()
	served := filepath.Join(base, "served")
	outside := filepath.Join(base, "outside")
	for _, dir := range []string{filepath.Join(served, "sub"), filepath.Join(outside, "leak")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(served, "public.txt"):     "PUBLIC",
		filepath.Join(served, "sub", "a.txt"):   "SUB",
		filepath.Join(outside, "secret.txt"):    "SECRET",
		filepath.Join(outside, "target.txt"):    "ORIGINAL",
		filepath.Join(outside, "leak", "x.txt"): "LEAK",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	links := map[string]string{
		"via":            "../outside",
		"link.txt":       "../outside/target.txt",
		"inner-link.txt": "public.txt",
		"inner-dir":      "sub",
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(served, name)); err != nil {
			t.Fatal(err)
		}
	}
	return NewLocal(served, false), outside
}

func TestLocalRejectsSymlinkEscape(t *testing.T) {
	fs, outside := newContainmentFixture(t)

	if f, err := fs.Open("link.txt"); err == nil {
		f.Close()
		t.Error("Open through final-segment symlink succeeded")
	}
	if _, err := fs.Stat("link.txt"); err == nil {
		t.Error("Stat through final-segment symlink succeeded")
	}
	if f, err := fs.Open("via/secret.txt"); err == nil {
		f.Close()
		t.Error("Open through intermediate symlink succeeded")
	}
	if _, err := fs.Stat("via/secret.txt"); err == nil {
		t.Error("Stat through intermediate symlink succeeded")
	}
	if _, err := fs.ReadDir("via/leak"); err == nil {
		t.Error("ReadDir through intermediate symlink succeeded")
	}

	if w, err := fs.Create("link.txt"); err == nil {
		_, _ = io.WriteString(w, "ATTACKER")
		w.Close()
		t.Error("Create through final-segment symlink succeeded")
	}
	if got, _ := os.ReadFile(filepath.Join(outside, "target.txt")); string(got) != "ORIGINAL" {
		t.Errorf("outside target.txt = %q, want ORIGINAL", got)
	}
	if w, err := fs.Create("via/new.txt"); err == nil {
		w.Close()
		t.Error("Create through intermediate symlink succeeded")
	}

	if err := fs.Mkdir("via/newdir", 0o755); err == nil {
		t.Error("Mkdir through intermediate symlink succeeded")
	}
	if _, err := os.Stat(filepath.Join(outside, "newdir")); !os.IsNotExist(err) {
		t.Errorf("outside newdir exists or stat failed: %v", err)
	}
	if err := fs.Remove("via/secret.txt"); err == nil {
		t.Error("Remove through intermediate symlink succeeded")
	}
	if got, _ := os.ReadFile(filepath.Join(outside, "secret.txt")); string(got) != "SECRET" {
		t.Errorf("outside secret.txt = %q, want SECRET", got)
	}
}

func TestLocalAllowsSymlinkInsideRoot(t *testing.T) {
	fs, _ := newContainmentFixture(t)

	f, err := fs.Open("inner-link.txt")
	if err != nil {
		t.Fatalf("Open inner-link.txt: %v", err)
	}
	got, err := io.ReadAll(f)
	f.Close()
	if err != nil || string(got) != "PUBLIC" {
		t.Fatalf("read inner-link.txt = %q, %v", got, err)
	}

	entries, err := fs.ReadDir("inner-dir")
	if err != nil || len(entries) != 1 || entries[0].Name() != "a.txt" {
		t.Fatalf("ReadDir inner-dir = %v, %v", entries, err)
	}

	w, err := fs.Create("inner-dir/new.txt")
	if err != nil {
		t.Fatalf("Create inner-dir/new.txt: %v", err)
	}
	w.Close()
	if err := fs.Mkdir("inner-dir/newdir", 0o755); err != nil {
		t.Fatalf("Mkdir inner-dir/newdir: %v", err)
	}
	if _, err := fs.Stat("sub/newdir"); err != nil {
		t.Fatalf("Stat sub/newdir: %v", err)
	}
	w, err = fs.Create("inner-link.txt")
	if err != nil {
		t.Fatalf("Create inner-link.txt: %v", err)
	}
	if _, err := io.WriteString(w, "UPDATED"); err != nil {
		t.Fatalf("write inner-link.txt: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close inner-link.txt: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(fs.Root(), "public.txt")); err != nil || string(got) != "UPDATED" {
		t.Fatalf("public.txt = %q, %v", got, err)
	}
	if err := fs.Remove("inner-dir/new.txt"); err != nil {
		t.Fatalf("Remove inner-dir/new.txt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(fs.Root(), "sub", "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("sub/new.txt exists or stat failed: %v", err)
	}
}
