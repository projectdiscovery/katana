package output

import (
	"os"
	"sync"
	"testing"
)

// TestCustomFieldsConcurrentAccess guards against the concurrent map access
// regression reported in issue #1698: initializing multiple writers while the
// custom fields map is read must not race.
func TestCustomFieldsConcurrentAccess(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "field-config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("- name: testfield\n  type: regex\n  part: response\n  regex: [\"(x)\"]\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w, err := New(Options{FieldConfig: f.Name()})
			if err == nil {
				_ = w.Close()
			}
		}()
		wg.Add(1)
		go func() {
			defer wg.Done()
			CustomFieldsSnapshot()
		}()
	}
	wg.Wait()
}
