package navigation

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/dominikbraun/graph/draw"
	"github.com/projectdiscovery/fileutil"
)

type Edge struct {
	From       *State
	To         *State
	Properties map[string]string
}

type GraphData struct {
	Vertexes []*State
	Edges    []Edge
}

func (g *Graph) ExportTo(outputFile string) error {
	basepath := filepath.Dir(outputFile)
	if !fileutil.FolderExists(basepath) {
		_ = fileutil.CreateFolder(basepath)
	}

	if err := g.ExportToDotFile(outputFile + ".dot"); err != nil {
		return err
	}

	if err := g.ExportToStructureFile(outputFile + ".dot"); err != nil {
		return err
	}

	return nil
}

func (g *Graph) ExportToDotFile(outputFile string) error {
	outputGraphFile, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer outputGraphFile.Close()

	return draw.DOT(g.graph, outputGraphFile)
}

func (g *Graph) ExportToStructureFile(outputFile string) error {
	outputGraphFile, err := os.Create(outputFile + ".json")
	if err != nil {
		return err
	}
	defer outputGraphFile.Close()

	data, err := json.Marshal(g.data)
	if err != nil {
		return err
	}

	_, err = outputGraphFile.Write(data)

	return err
}

func (g *Graph) ImportFromStructureFile(inputFile string) error {
	f, err := os.Open(inputFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	return json.NewDecoder(f).Decode(&g.data)
}

func (g *Graph) RefreshStructure(inputFile string) error {
	// re-add all the vertexes
	for _, vertex := range g.data.Vertexes {
		if err := g.graph.AddVertex(*vertex); err != nil {
			return err
		}
	}

	// re-add all the edges
	for _, edge := range g.data.Edges {
		if err := g.graph.AddEdge(StateHash(*edge.From), StateHash(*edge.To), g.toEdgeProperties(edge.Properties)...); err != nil {
			return err
		}
	}

	return nil
}
