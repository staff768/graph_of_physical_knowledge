// Package content загружает и валидирует граф знаний из YAML-файлов.
package content

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"graph_of_physical_knowledge/internal/domain"
)

type file struct {
	Section string        `yaml:"section"`
	Nodes   []domain.Node `yaml:"nodes"`
	Edges   []domain.Edge `yaml:"edges"`
}

// LoadDir читает все *.yaml файлы каталога, объединяет их в один граф
// и валидирует результат.
func LoadDir(fsys fs.FS, dir string) (domain.Graph, error) {
	names, err := fs.Glob(fsys, path.Join(dir, "*.yaml"))
	if err != nil {
		return domain.Graph{}, err
	}
	if len(names) == 0 {
		return domain.Graph{}, fmt.Errorf("no *.yaml files in %s", dir)
	}
	sort.Strings(names)
	var g domain.Graph
	for _, name := range names {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return domain.Graph{}, err
		}
		var f file
		dec := yaml.NewDecoder(strings.NewReader(string(data)))
		dec.KnownFields(true)
		if err := dec.Decode(&f); err != nil {
			return domain.Graph{}, fmt.Errorf("%s: %w", name, err)
		}
		for _, n := range f.Nodes {
			if n.Section == "" {
				n.Section = f.Section
			}
			g.Nodes = append(g.Nodes, n)
		}
		g.Edges = append(g.Edges, f.Edges...)
	}
	if err := Validate(g); err != nil {
		return domain.Graph{}, err
	}
	return g, nil
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// allowedEdges задаёт допустимые пары (тип основания → тип следствия) для каждого типа ребра.
var allowedEdges = map[domain.EdgeType][][2]domain.NodeType{
	domain.EdgeFormulates: {{domain.NodeLaw, domain.NodeFormula}},
	domain.EdgeUsedIn:     {{domain.NodeQuantity, domain.NodeFormula}, {domain.NodeQuantity, domain.NodeLaw}},
	domain.EdgeDerives:    {{domain.NodeFormula, domain.NodeFormula}, {domain.NodeLaw, domain.NodeLaw}},
	domain.EdgeExplains: {
		{domain.NodeLaw, domain.NodePhenomenon}, {domain.NodeFormula, domain.NodePhenomenon},
	},
	domain.EdgeCauses:  {{domain.NodePhenomenon, domain.NodePhenomenon}},
	domain.EdgeDefines: {{domain.NodeLaw, domain.NodeQuantity}, {domain.NodeFormula, domain.NodeQuantity}},
}

// Validate проверяет целостность графа: корректность slug и типов,
// существование концов рёбер, допустимость связей и отсутствие циклов.
func Validate(g domain.Graph) error {
	var errs []error
	nodeType := make(map[string]domain.NodeType, len(g.Nodes))
	for _, n := range g.Nodes {
		switch {
		case !slugRe.MatchString(n.Slug):
			errs = append(errs, fmt.Errorf("node %q: invalid slug", n.Slug))
		case !n.Type.Valid():
			errs = append(errs, fmt.Errorf("node %q: unknown type %q", n.Slug, n.Type))
		case strings.TrimSpace(n.Title) == "":
			errs = append(errs, fmt.Errorf("node %q: empty title", n.Slug))
		}
		if _, dup := nodeType[n.Slug]; dup {
			errs = append(errs, fmt.Errorf("node %q: duplicate slug", n.Slug))
		}
		nodeType[n.Slug] = n.Type
	}

	type edgeKey struct {
		from, to string
		typ      domain.EdgeType
	}
	seen := make(map[edgeKey]bool, len(g.Edges))
	for _, e := range g.Edges {
		name := fmt.Sprintf("edge %s -[%s]-> %s", e.From, e.Type, e.To)
		fromType, okFrom := nodeType[e.From]
		toType, okTo := nodeType[e.To]
		switch {
		case !e.Type.Valid():
			errs = append(errs, fmt.Errorf("%s: unknown type", name))
			continue
		case !okFrom || !okTo:
			errs = append(errs, fmt.Errorf("%s: references unknown node", name))
			continue
		case e.From == e.To:
			errs = append(errs, fmt.Errorf("%s: self-loop", name))
			continue
		}
		key := edgeKey{e.From, e.To, e.Type}
		if seen[key] {
			errs = append(errs, fmt.Errorf("%s: duplicate", name))
		}
		seen[key] = true
		if !edgeAllowed(e.Type, fromType, toType) {
			errs = append(errs, fmt.Errorf("%s: not allowed between %s and %s", name, fromType, toType))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	if cycle := findCycle(g); cycle != nil {
		return fmt.Errorf("graph has a cycle: %s", strings.Join(cycle, " -> "))
	}
	return nil
}

func edgeAllowed(t domain.EdgeType, from, to domain.NodeType) bool {
	for _, pair := range allowedEdges[t] {
		if pair[0] == from && pair[1] == to {
			return true
		}
	}
	return false
}

// findCycle возвращает один найденный цикл или nil, если граф ацикличен.
func findCycle(g domain.Graph) []string {
	out := make(map[string][]string)
	for _, e := range g.Edges {
		out[e.From] = append(out[e.From], e.To)
	}
	const (
		white = iota
		grey
		black
	)
	color := make(map[string]int)
	var stack []string
	var visit func(string) []string
	visit = func(v string) []string {
		color[v] = grey
		stack = append(stack, v)
		for _, w := range out[v] {
			switch color[w] {
			case grey:
				for i, s := range stack {
					if s == w {
						return append(append([]string{}, stack[i:]...), w)
					}
				}
			case white:
				if c := visit(w); c != nil {
					return c
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[v] = black
		return nil
	}
	for _, n := range g.Nodes {
		if color[n.Slug] == white {
			if c := visit(n.Slug); c != nil {
				return c
			}
		}
	}
	return nil
}
