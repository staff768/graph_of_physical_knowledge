// Команда eval прогоняет конспект через языковую модель из .env и оценивает граф:
// какие ожидаемые понятия нашлись, сколько опирается на цитаты, поймана ли ошибка в формуле,
// сколько ушло токенов и времени.
//
//	go run ./cmd/eval                                   # тестовый конспект по электричеству
//	go run ./cmd/eval -notes my.pdf -expected my.yaml   # свой конспект
//	go run ./cmd/eval -price-in 15 -price-out 60        # оценка стоимости, цена за 1M токенов
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"

	"graph_of_physical_knowledge/internal/builder"
	"graph_of_physical_knowledge/internal/content"
	"graph_of_physical_knowledge/internal/document"
	"graph_of_physical_knowledge/internal/domain"
	"graph_of_physical_knowledge/internal/llm"
)

type expectations struct {
	Concepts [][]string `yaml:"concepts"`
	Warnings [][]string `yaml:"warnings"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "eval:", err)
		os.Exit(1)
	}
}

func run() error {
	notesPath := flag.String("notes", "testdata/notes/electricity.txt", "конспект (PDF, DOCX, TXT, JPG, PNG)")
	expectedPath := flag.String("expected", "testdata/notes/electricity.expected.yaml", "ожидаемые понятия")
	out := flag.String("out", "", "сохранить построенный граф в JSON-файл")
	priceIn := flag.Float64("price-in", 0, "цена 1M входных токенов (для оценки стоимости)")
	priceOut := flag.Float64("price-out", 0, "цена 1M выходных токенов")
	flag.Parse()
	_ = godotenv.Load()

	client, err := llm.FromEnv()
	if err != nil {
		return fmt.Errorf("модель не настроена: %w (заполните .env по образцу .env.example)", err)
	}
	data, err := os.ReadFile(*notesPath)
	if err != nil {
		return err
	}
	kind, err := document.Kind(*notesPath, data)
	if err != nil {
		return err
	}
	doc, err := document.Parse(kind, data)
	if err != nil {
		return err
	}
	var exp expectations
	if raw, err := os.ReadFile(*expectedPath); err == nil {
		if err := yaml.Unmarshal(raw, &exp); err != nil {
			return fmt.Errorf("%s: %w", *expectedPath, err)
		}
	}

	fmt.Printf("Модель: %s\nКонспект: %s (%d стр.)\n\n", client.Name(), *notesPath, doc.PageCount())
	start := time.Now()
	b := &builder.Builder{LLM: client, CheckPhysics: true}
	res, err := b.Build(context.Background(), doc, func(stage, note string) {
		fmt.Printf("  [%5.1fs] %s %s\n", time.Since(start).Seconds(), stage, note)
	})
	if err != nil {
		return err
	}
	elapsed := time.Since(start)
	g := res.Graph

	fmt.Printf("\nГраф «%s» (%s): %d понятий, %d связей\n", res.Title, res.Section, len(g.Nodes), len(g.Edges))
	if err := content.Validate(g); err != nil {
		fmt.Printf("  ✗ граф не прошёл проверку: %v\n", err)
	} else {
		fmt.Println("  ✓ граф прошёл проверку структуры")
	}

	explicitNodes, inferredEdges := 0, 0
	for _, n := range g.Nodes {
		if n.Origin == domain.OriginExplicit {
			explicitNodes++
		}
	}
	for _, e := range g.Edges {
		if e.Origin == domain.OriginInferred {
			inferredEdges++
		}
	}
	fmt.Printf("  понятий с цитатой из конспекта: %d из %d (%.0f%%)\n", explicitNodes, len(g.Nodes), pct(explicitNodes, len(g.Nodes)))
	fmt.Printf("  связей, дополненных моделью:    %d из %d (%.0f%%)\n", inferredEdges, len(g.Edges), pct(inferredEdges, len(g.Edges)))
	fmt.Printf("  автоисправлений:                %d\n", len(res.Fixes))
	for _, f := range res.Fixes {
		fmt.Printf("    · %s\n", f)
	}

	if len(exp.Concepts) > 0 {
		var missing []string
		found := 0
		for _, variants := range exp.Concepts {
			if findNode(g, variants, false) != nil {
				found++
			} else {
				missing = append(missing, variants[0])
			}
		}
		fmt.Printf("\nОжидаемые понятия: найдено %d из %d (%.0f%%)\n", found, len(exp.Concepts), pct(found, len(exp.Concepts)))
		if len(missing) > 0 {
			fmt.Printf("  не найдены: %s\n", strings.Join(missing, ", "))
		}
		extra := 0
		for _, n := range g.Nodes {
			if !matchesAny(n, exp.Concepts) {
				extra++
			}
		}
		fmt.Printf("  понятий сверх ожидаемых: %d (не обязательно ошибки — проверьте глазами)\n", extra)
	}
	for _, variants := range exp.Warnings {
		if n := findNode(g, variants, true); n != nil {
			fmt.Printf("  ✓ ошибка в формуле поймана: «%s» — %s\n", n.Title, n.Warning)
		} else {
			fmt.Printf("  ✗ ошибка в формуле не поймана (%s)\n", variants[0])
		}
	}
	for _, n := range g.Nodes {
		if n.Warning != "" && !matchesAnyWarning(n, exp.Warnings) {
			fmt.Printf("  ? лишнее предупреждение: «%s» — %s\n", n.Title, n.Warning)
		}
	}

	fmt.Printf("\nТокены: %d вход + %d выход, время: %s\n", res.Usage.InputTokens, res.Usage.OutputTokens, elapsed.Round(100*time.Millisecond))
	if *priceIn > 0 || *priceOut > 0 {
		cost := float64(res.Usage.InputTokens)/1e6**priceIn + float64(res.Usage.OutputTokens)/1e6**priceOut
		fmt.Printf("Стоимость конспекта: %.4f (в валюте цен)\n", cost)
	}
	if *out != "" {
		raw, err := json.MarshalIndent(g, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(*out, raw, 0o644); err != nil {
			return err
		}
		fmt.Printf("Граф сохранён в %s\n", *out)
	}
	return nil
}

// findNode ищет узел, название которого совпадает с одним из вариантов.
// withWarning — только среди узлов с предупреждением, сравнивая также формулу.
func findNode(g domain.Graph, variants []string, withWarning bool) *domain.Node {
	for i, n := range g.Nodes {
		if withWarning && n.Warning == "" {
			continue
		}
		for _, v := range variants {
			if similar(n.Title, v) || (withWarning && similar(n.Formula, v)) {
				return &g.Nodes[i]
			}
		}
	}
	return nil
}

func matchesAny(n domain.Node, groups [][]string) bool {
	for _, variants := range groups {
		for _, v := range variants {
			if similar(n.Title, v) {
				return true
			}
		}
	}
	return false
}

func matchesAnyWarning(n domain.Node, groups [][]string) bool {
	for _, variants := range groups {
		for _, v := range variants {
			if similar(n.Title, v) || similar(n.Formula, v) {
				return true
			}
		}
	}
	return false
}

// similar: одна строка содержит другую без учёта регистра, пробелов и тире.
func similar(a, b string) bool {
	a, b = key(a), key(b)
	if a == "" || b == "" {
		return false
	}
	return strings.Contains(a, b) || strings.Contains(b, a)
}

func key(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsSpace(r), r == '-', r == '–', r == '—':
		case r == 'ё':
			sb.WriteRune('е')
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func pct(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return 100 * float64(a) / float64(b)
}
