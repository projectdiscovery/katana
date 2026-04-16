package browser

import (
	"fmt"
	"strings"

	"github.com/go-rod/rod/lib/proto"
)

// GetAccessibilityTree returns a compact text representation of the page's
// accessibility tree via the CDP Accessibility domain.
//
// The output is an indented tree with format:
//
//	[role] "name" {properties}
//
// This is 10-50x more compact than raw DOM and contains semantic information
// (roles, labels, states) that raw HTML doesn't surface clearly.
//
// maxDepth controls how deep the tree is fetched. 0 means the full tree.
func (bp *BrowserPage) GetAccessibilityTree(maxDepth int) (string, error) {
	req := proto.AccessibilityGetFullAXTree{}
	if maxDepth > 0 {
		req.Depth = &maxDepth
	}

	result, err := req.Call(bp.Page)
	if err != nil {
		return "", fmt.Errorf("accessibility tree: %w", err)
	}

	if len(result.Nodes) == 0 {
		return "", nil
	}

	return formatAXTree(result.Nodes), nil
}

// formatAXTree converts CDP accessibility nodes into an indented text tree.
// It builds a parent→children map, then renders depth-first from the root.
func formatAXTree(nodes []*proto.AccessibilityAXNode) string {
	if len(nodes) == 0 {
		return ""
	}

	// Build lookup maps
	nodeByID := make(map[proto.AccessibilityAXNodeID]*proto.AccessibilityAXNode, len(nodes))
	for _, n := range nodes {
		nodeByID[n.NodeID] = n
	}

	var b strings.Builder
	// The first node is typically the root (document)
	renderAXNode(&b, nodes[0], nodeByID, 0)
	return b.String()
}

// renderAXNode recursively renders a node and its children.
func renderAXNode(b *strings.Builder, node *proto.AccessibilityAXNode, lookup map[proto.AccessibilityAXNodeID]*proto.AccessibilityAXNode, depth int) {
	if node.Ignored {
		// Skip ignored nodes but still render their children
		// (some ignored containers have meaningful children)
		for _, childID := range node.ChildIDs {
			if child, ok := lookup[childID]; ok {
				renderAXNode(b, child, lookup, depth)
			}
		}
		return
	}

	role := axValueString(node.Role)
	name := axValueString(node.Name)

	// Skip generic containers with no name — they add noise without information.
	// But still render their children.
	if isGenericRole(role) && name == "" {
		for _, childID := range node.ChildIDs {
			if child, ok := lookup[childID]; ok {
				renderAXNode(b, child, lookup, depth)
			}
		}
		return
	}

	// Build the line
	indent := strings.Repeat("  ", depth)
	b.WriteString(indent)
	b.WriteByte('[')
	b.WriteString(role)
	b.WriteByte(']')

	if name != "" {
		b.WriteString(` "`)
		// Truncate very long names (e.g., paragraph text)
		if len(name) > 80 {
			name = name[:77] + "..."
		}
		b.WriteString(name)
		b.WriteByte('"')
	}

	// Append notable properties
	props := extractNotableProperties(node)
	if props != "" {
		b.WriteByte(' ')
		b.WriteString(props)
	}

	b.WriteByte('\n')

	// Render children
	for _, childID := range node.ChildIDs {
		if child, ok := lookup[childID]; ok {
			renderAXNode(b, child, lookup, depth+1)
		}
	}
}

// axValueString extracts a string from an AccessibilityAXValue.
func axValueString(v *proto.AccessibilityAXValue) string {
	if v == nil {
		return ""
	}
	// v.Value is gson.JSON — try to get it as a string
	s := v.Value.Str()
	if s != "" {
		return s
	}
	return v.Value.String()
}

// isGenericRole returns true for container roles that are noisy without a name.
func isGenericRole(role string) bool {
	switch role {
	case "generic", "none", "group", "paragraph", "Section",
		"LayoutTable", "LayoutTableRow", "LayoutTableCell",
		"LineBreak", "RootWebArea", "":
		return true
	}
	return false
}

// extractNotableProperties returns a compact string of important ARIA properties.
func extractNotableProperties(node *proto.AccessibilityAXNode) string {
	var parts []string
	for _, prop := range node.Properties {
		switch prop.Name {
		case "required":
			if prop.Value != nil && prop.Value.Value.Bool() {
				parts = append(parts, "required")
			}
		case "disabled":
			if prop.Value != nil && prop.Value.Value.Bool() {
				parts = append(parts, "disabled")
			}
		case "expanded":
			if prop.Value != nil {
				if prop.Value.Value.Bool() {
					parts = append(parts, "expanded")
				} else {
					parts = append(parts, "collapsed")
				}
			}
		case "checked":
			if prop.Value != nil {
				s := prop.Value.Value.Str()
				if s == "true" {
					parts = append(parts, "checked")
				} else if s == "mixed" {
					parts = append(parts, "indeterminate")
				}
			}
		case "focused":
			if prop.Value != nil && prop.Value.Value.Bool() {
				parts = append(parts, "focused")
			}
		case "readonly":
			if prop.Value != nil && prop.Value.Value.Bool() {
				parts = append(parts, "readonly")
			}
		}
	}
	return strings.Join(parts, " ")
}
