package diagnostics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	mapsutil "github.com/projectdiscovery/utils/maps"
)

// Writer is a writer that writes diagnostics to a directory
// for the katana headless crawler module.
type Writer interface {
	Close() error
	LogAction(action *types.Action) error
	LogPageState(state *types.PageState, stateType PageStateType) error
}

type PageStateType string

var (
	PreActionPageState  PageStateType = "pre-action"
	PostActionPageState PageStateType = "post-action"
)

type diskWriter struct {
	index     mapsutil.OrderedMap[string, *stateMetadata]
	actions   []*types.Action
	mu        sync.Mutex
	directory string
}

type stateMetadata struct {
	UniqueID  string `json:"unique_id"`
	URL       string `json:"url"`
	Title     string `json:"title"`
	Occurence int    `json:"occurence"`
	Type      string `json:"type"`
}

// NewWriter creates a new Writer.
func NewWriter(directory string) (Writer, error) {
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
	}
	return &diskWriter{
		directory: directory,
		index:     mapsutil.NewOrderedMap[string, *stateMetadata](),
		actions:   make([]*types.Action, 0),
		mu:        sync.Mutex{},
	}, nil
}

func (w *diskWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	actionsList := w.actions
	marshallIndented, err := json.MarshalIndent(actionsList, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(w.directory, "actions.json"), marshallIndented, 0644); err != nil {
		return err
	}

	// Write index to a separate file
	var data []*stateMetadata
	w.index.Iterate(func(key string, value *stateMetadata) bool {
		data = append(data, value)
		return true
	})

	marshallIndented, err = json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(w.directory, "index.json"), marshallIndented, 0644)
}

func (w *diskWriter) LogAction(action *types.Action) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.actions = append(w.actions, action)
	return nil
}

func (w *diskWriter) LogPageState(state *types.PageState, stateType PageStateType) error {
	w.mu.Lock()
	val, ok := w.index.Get(state.UniqueID)
	if ok && val != nil {
		w.mu.Unlock()
		val.Occurence++
		return nil
	}
	w.index.Set(state.UniqueID, &stateMetadata{
		URL:       state.URL,
		Title:     state.Title,
		Occurence: 1,
		Type:      string(stateType),
		UniqueID:  state.UniqueID,
	})
	w.mu.Unlock()

	// Write dom to a separate file and remove striped dom
	// Create new directory for each state
	dom, strippedDOM := state.DOM, state.StrippedDOM
	state.DOM, state.StrippedDOM = "", ""

	dir := filepath.Join(w.directory, state.UniqueID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	domFile := filepath.Join(dir, "dom.html")
	if err := os.WriteFile(domFile, []byte(dom), 0644); err != nil {
		return err
	}
	strippedDOMFile := filepath.Join(dir, "stripped-dom.html")
	if err := os.WriteFile(strippedDOMFile, []byte(strippedDOM), 0644); err != nil {
		return err
	}
	return nil
}
