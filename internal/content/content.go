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
	Graph   *domain.GraphMeta `yaml:"graph"`
	Section string            `yaml:"section"`
	Nodes   []domain.Node     `yaml:"nodes"`
	Edges   []domain.Edge     `yaml:"edges"`
}

// Bundle — граф вместе со сведениями о нём.
type Bundle struct {
	Meta  domain.GraphMeta
	Graph domain.Graph
}

// LoadDir читает все *.yaml файлы каталога, объединяет их в один граф
// и валидирует результат. Сведения о графе задаются блоком graph: ровно в одном файле.
func LoadDir(fsys fs.FS, dir string) (Bundle, error) {
	names, err := fs.Glob(fsys, path.Join(dir, "*.yaml"))
	if err != nil {
		return Bundle{}, err
	}
	if len(names) == 0 {
		return Bundle{}, fmt.Errorf("no *.yaml files in %s", dir)
	}
	sort.Strings(names)
	var g domain.Graph
	var meta *domain.GraphMeta
	for _, name := range names {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return Bundle{}, err
		}
		var f file
		dec := yaml.NewDecoder(strings.NewReader(string(data)))
		dec.KnownFields(true)
		if err := dec.Decode(&f); err != nil {
			return Bundle{}, fmt.Errorf("%s: %w", name, err)
		}
		if f.Graph != nil {
			if meta != nil {
				return Bundle{}, fmt.Errorf("%s: graph block is already defined in another file", name)
			}
			meta = f.Graph
		}
		for _, n := range f.Nodes {
			if n.Section == "" {
				n.Section = f.Section
			}
			g.Nodes = append(g.Nodes, n)
		}
		g.Edges = append(g.Edges, f.Edges...)
	}
	if meta == nil {
		return Bundle{}, fmt.Errorf("no graph block (id, title) in %s", dir)
	}
	if !slugRe.MatchString(meta.ID) || strings.TrimSpace(meta.Title) == "" {
		return Bundle{}, fmt.Errorf("graph block: id must be a slug and title must be set")
	}
	if err := Validate(g); err != nil {
		return Bundle{}, err
	}
	return Bundle{Meta: *meta, Graph: g}, nil
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
	domain.EdgeCauses: {{domain.NodePhenomenon, domain.NodePhenomenon}},
	domain.EdgeDefines: {
		{domain.NodeLaw, domain.NodeQuantity}, {domain.NodeFormula, domain.NodeQuantity},
		{domain.NodePhenomenon, domain.NodeQuantity}, // явление вводит величину: ток → сила тока
	},
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
		if !EdgeAllowed(e.Type, fromType, toType) {
			errs = append(errs, fmt.Errorf("%s: not allowed between %s and %s", name, fromType, toType))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	if cycle := FindCycle(g); cycle != nil {
		return fmt.Errorf("graph has a cycle: %s", strings.Join(cycle, " -> "))
	}
	return nil
}

// EdgeAllowed сообщает, допустима ли связь типа t между узлами этих типов.
func EdgeAllowed(t domain.EdgeType, from, to domain.NodeType) bool {
	for _, pair := range allowedEdges[t] {
		if pair[0] == from && pair[1] == to {
			return true
		}
	}
	return false
}

// FindCycle возвращает один найденный цикл (slug узлов, первый повторён в конце)
// или nil, если граф ацикличен.
func FindCycle(g domain.Graph) []string {
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
