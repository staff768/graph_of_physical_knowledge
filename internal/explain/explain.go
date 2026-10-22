// Package explain строит структурированный разбор физического процесса
// по подграфу предков выбранного узла.
package explain

import (
	"fmt"
	"sort"
	"strings"

	"graph_of_physical_knowledge/internal/domain"
)

// Reason — одно основание шага: входящее ребро от другого узла цепочки.
type Reason struct {
	From      string          `json:"from"`
	FromTitle string          `json:"fromTitle"`
	Type      domain.EdgeType `json:"type"`
	// Text — человекочитаемая фраза, например «Использует величину «Сила тока»».
	Text        string `json:"text"`
	Description string `json:"description,omitempty"`
}

// Step — один шаг разбора. Шаги упорядочены так, что все основания шага
// встречаются раньше него самого.
type Step struct {
	Index int         `json:"index"`
	Level int         `json:"level"`
	Node  domain.Node `json:"node"`
	// Reasons пуст для исходных узлов (законов и величин без предпосылок).
	Reasons []Reason `json:"reasons"`
}

type Explanation struct {
	Target domain.Node `json:"target"`
	// Overview — короткий связный текст-введение к разбору.
	Overview string `json:"overview"`
	// CausalChain — цепочка явлений, причинно приводящая к цели (включая её),
	// если цель — явление и у неё есть причины.
	CausalChain []string `json:"causalChain"`
	Steps       []Step   `json:"steps"`
	// Levels — количество уровней (глубина) цепочки.
	Levels int          `json:"levels"`
	Graph  domain.Graph `json:"graph"`
}

var reasonTemplates = map[domain.EdgeType]string{
	domain.EdgeFormulates: "Математическая запись закона «%s»",
	domain.EdgeUsedIn:     "Использует величину «%s»",
	domain.EdgeDerives:    "Выводится из «%s»",
	domain.EdgeExplains:   "Объясняется через «%s»",
	domain.EdgeCauses:     "Вызвано явлением «%s»",
	domain.EdgeDefines:    "Величина вводится через «%s»",
}

// ReasonText формирует фразу-основание для ребра с точки зрения узла-следствия.
func ReasonText(t domain.EdgeType, fromTitle string) string {
	tmpl, ok := reasonTemplates[t]
	if !ok {
		tmpl = "Связано с «%s»"
	}
	return fmt.Sprintf(tmpl, fromTitle)
}

var typeOrder = map[domain.NodeType]int{
	domain.NodeLaw:        0,
	domain.NodeQuantity:   1,
	domain.NodeFormula:    2,
	domain.NodePhenomenon: 3,
}

// Build строит разбор для узла target по подграфу его предков sub
// (sub должен содержать сам target).
func Build(target string, sub domain.Graph) (Explanation, error) {
	byslug := make(map[string]domain.Node, len(sub.Nodes))
	for _, n := range sub.Nodes {
		byslug[n.Slug] = n
	}
	targetNode, ok := byslug[target]
	if !ok {
		return Explanation{}, fmt.Errorf("target %q is not in subgraph", target)
	}

	incoming := make(map[string][]domain.Edge)
	outgoing := make(map[string][]domain.Edge)
	for _, e := range sub.Edges {
		incoming[e.To] = append(incoming[e.To], e)
		outgoing[e.From] = append(outgoing[e.From], e)
	}

	// Все узлы подграфа — предки цели, поэтому цель оказывается
	// на последнем уровне и в конце разбора.
	level := longestPathLevels(sub.Nodes, incoming, outgoing)

	nodes := append([]domain.Node(nil), sub.Nodes...)
	sort.SliceStable(nodes, func(i, j int) bool {
		a, b := nodes[i], nodes[j]
		if level[a.Slug] != level[b.Slug] {
			return level[a.Slug] < level[b.Slug]
		}
		if typeOrder[a.Type] != typeOrder[b.Type] {
			return typeOrder[a.Type] < typeOrder[b.Type]
		}
		return a.Title < b.Title
	})

	steps := make([]Step, 0, len(nodes))
	for i, n := range nodes {
		reasons := make([]Reason, 0, len(incoming[n.Slug]))
		for _, e := range incoming[n.Slug] {
			from := byslug[e.From]
			reasons = append(reasons, Reason{
				From:        from.Slug,
				FromTitle:   from.Title,
				Type:        e.Type,
				Text:        ReasonText(e.Type, from.Title),
				Description: e.Description,
			})
		}
		sort.SliceStable(reasons, func(a, b int) bool {
			return typeOrder[byslug[reasons[a].From].Type] < typeOrder[byslug[reasons[b].From].Type]
		})
		steps = append(steps, Step{Index: i + 1, Level: level[n.Slug], Node: n, Reasons: reasons})
	}

	chain := causalChain(target, incoming, level)
	return Explanation{
		Target:      targetNode,
		Overview:    overview(targetNode, nodes, chain, byslug, incoming),
		CausalChain: chain,
		Steps:       steps,
		Levels:      level[target] + 1,
		Graph:       sub,
	}, nil
}

// longestPathLevels присваивает каждому узлу уровень — длину самого длинного
// пути до него от узла без входящих рёбер (алгоритм Кана).
func longestPathLevels(nodes []domain.Node, incoming, outgoing map[string][]domain.Edge) map[string]int {
	level := make(map[string]int, len(nodes))
	indeg := make(map[string]int, len(nodes))
	var queue []string
	for _, n := range nodes {
		indeg[n.Slug] = len(incoming[n.Slug])
		if indeg[n.Slug] == 0 {
			queue = append(queue, n.Slug)
		}
	}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		for _, e := range outgoing[v] {
			level[e.To] = max(level[e.To], level[v]+1)
			indeg[e.To]--
			if indeg[e.To] == 0 {
				queue = append(queue, e.To)
			}
		}
	}
	return level
}

// causalChain восстанавливает самую длинную цепочку явлений по рёбрам causes,
// заканчивающуюся в target. Возвращает nil, если причин у цели нет.
func causalChain(target string, incoming map[string][]domain.Edge, level map[string]int) []string {
	chain := []string{target}
	visited := map[string]bool{target: true}
	cur := target
	for {
		next := ""
		for _, e := range incoming[cur] {
			if e.Type != domain.EdgeCauses || visited[e.From] {
				continue
			}
			if next == "" || level[e.From] > level[next] {
				next = e.From
			}
		}
		if next == "" {
			break
		}
		visited[next] = true
		chain = append(chain, next)
		cur = next
	}
	if len(chain) == 1 {
		return nil
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}

func overview(target domain.Node, nodes []domain.Node, chain []string, byslug map[string]domain.Node, incoming map[string][]domain.Edge) string {
	var baseLaws, derivedLaws, formulas []string
	for _, n := range nodes {
		if n.Slug == target.Slug {
			continue
		}
		switch n.Type {
		case domain.NodeLaw:
			if len(incoming[n.Slug]) == 0 {
				baseLaws = append(baseLaws, "«"+n.Title+"»")
			} else {
				derivedLaws = append(derivedLaws, "«"+n.Title+"»")
			}
		case domain.NodeFormula:
			formulas = append(formulas, "«"+n.Title+"»")
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "«%s»", target.Title)
	if len(nodes) == 1 {
		b.WriteString(" — исходное понятие графа: у него нет предпосылок.")
		return b.String()
	}
	fmt.Fprintf(&b, " опирается на %d %s.", len(nodes)-1, plural(len(nodes)-1, "понятие", "понятия", "понятий"))
	if len(baseLaws) > 0 {
		fmt.Fprintf(&b, " Фундамент: %s.", strings.Join(baseLaws, ", "))
	}
	if len(derivedLaws) > 0 {
		fmt.Fprintf(&b, " Из них следуют: %s.", strings.Join(derivedLaws, ", "))
	}
	if len(formulas) > 0 {
		fmt.Fprintf(&b, " Ключевые соотношения: %s.", strings.Join(formulas, ", "))
	}
	if len(chain) > 1 {
		titles := make([]string, len(chain))
		for i, s := range chain {
			titles[i] = byslug[s].Title
		}
		fmt.Fprintf(&b, " Причинная цепочка: %s.", strings.Join(titles, " → "))
	}
	return b.String()
}

func plural(n int, one, few, many string) string {
	n = n % 100
	if n >= 11 && n <= 14 {
		return many
	}
	switch n % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	}
	return many
}
