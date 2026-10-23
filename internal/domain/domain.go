// Package domain описывает сущности графа физических знаний.
package domain

import "time"

// NodeType — тип узла графа.
type NodeType string

const (
	NodeLaw        NodeType = "law"        // фундаментальный закон или принцип
	NodeQuantity   NodeType = "quantity"   // физическая величина
	NodeFormula    NodeType = "formula"    // математическое соотношение
	NodePhenomenon NodeType = "phenomenon" // природное или техническое явление
)

func (t NodeType) Valid() bool {
	switch t {
	case NodeLaw, NodeQuantity, NodeFormula, NodePhenomenon:
		return true
	}
	return false
}

// EdgeType — тип связи. Ребро всегда направлено от более фундаментального
// узла (основания) к зависимому (следствию).
type EdgeType string

const (
	EdgeFormulates EdgeType = "formulates" // закон → формула: закон записывается формулой
	EdgeUsedIn     EdgeType = "used_in"    // величина → формула: величина входит в формулу
	EdgeDerives    EdgeType = "derives"    // формула → формула: одна выводится из другой
	EdgeExplains   EdgeType = "explains"   // закон/формула → явление: объясняет явление
	EdgeCauses     EdgeType = "causes"     // явление → явление: причинно-следственная связь
	EdgeDefines    EdgeType = "defines"    // закон/формула → величина: вводит величину
)

func (t EdgeType) Valid() bool {
	switch t {
	case EdgeFormulates, EdgeUsedIn, EdgeDerives, EdgeExplains, EdgeCauses, EdgeDefines:
		return true
	}
	return false
}

type Node struct {
	Slug    string   `json:"slug"`
	Type    NodeType `json:"type"`
	Title   string   `json:"title"`
	Section string   `json:"section"`
	Summary string   `json:"summary"`
	// Formula — запись в LaTeX (для формул, законов и величин).
	Formula string `json:"formula,omitempty"`
	// Unit — единица измерения в LaTeX (для величин).
	Unit string `json:"unit,omitempty"`
	// Body — подробное описание в Markdown с LaTeX-вставками.
	Body string `json:"body,omitempty"`
	// Example — числовой пример в Markdown с LaTeX-вставками.
	Example string `json:"example,omitempty"`
}

type Edge struct {
	From        string   `json:"from"`
	To          string   `json:"to"`
	Type        EdgeType `json:"type"`
	Description string   `json:"description,omitempty"`
}

// GraphMeta — сведения о графе для библиотеки.
type GraphMeta struct {
	ID        string    `json:"id" yaml:"id"`
	Title     string    `json:"title" yaml:"title"`
	Section   string    `json:"section" yaml:"section"`
	Source    string    `json:"source" yaml:"source"`
	Demo      bool      `json:"demo" yaml:"demo"`
	CreatedAt time.Time `json:"createdAt" yaml:"-"`
	NodeCount int       `json:"nodeCount" yaml:"-"`
	EdgeCount int       `json:"edgeCount" yaml:"-"`
	// Preview — несколько самых связанных понятий графа для превью на карточке.
	Preview []string `json:"preview" yaml:"-"`
}

// Link — связь узла с соседом, вместе с краткими данными соседа.
type Link struct {
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	NodeType    NodeType `json:"nodeType"`
	EdgeType    EdgeType `json:"edgeType"`
	Description string   `json:"description,omitempty"`
}

type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}
