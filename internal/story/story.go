// Package story пишет связный рассказ по цепочке разбора с помощью языковой модели:
// почему из законов и величин получается выбранное явление — простыми словами для школьника.
package story

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"graph_of_physical_knowledge/internal/domain"
	"graph_of_physical_knowledge/internal/explain"
	"graph_of_physical_knowledge/internal/llm"
)

const systemPrompt = `Ты — учитель физики, который объясняет школьнику 8–11 класса, почему происходит явление.
Тебе дана цепочка понятий из графа знаний: законы, величины, формулы и явления, упорядоченные от оснований к цели, и связи между ними.

Напиши связный рассказ о том, как из оснований получается цель:
- 2–4 коротких абзаца, всего 120–220 слов, без заголовков и списков;
- иди от оснований к цели и объясняй, почему каждый шаг ведёт к следующему («поэтому», «а значит»), а не просто перечисляй понятия;
- опирайся только на понятия из цепочки, новых законов не добавляй; можно пропустить второстепенные шаги;
- каждый качественный вывод («больше», «меньше», «растёт») сверяй с формулой: например, по $R = \\rho l / S$ длинный и тонкий провод имеет большое сопротивление; если не уверен — не делай вывода;
- формулы пиши в LaTeX внутри $...$, например $Q = I^2 R t$; названия понятий выделяй **жирным** при первом упоминании;
- если у понятия есть пометка «ошибка в конспекте», используй верную формулу и одной фразой предупреди об ошибке;
- закончи одним предложением с примером из жизни, если он уместен.
Пиши по-русски, просто и точно, без вступлений вроде «Давай разберёмся».`

// MaxTokens — лимит ответа: рассказ короткий, запас на случай многословной модели.
const MaxTokens = 2000

// ErrEmpty — модель вернула пустой ответ.
var ErrEmpty = errors.New("model returned an empty story")

// Teller пишет рассказы.
type Teller struct {
	LLM llm.Client
}

// Tell пишет рассказ по разбору exp.
func (t *Teller) Tell(ctx context.Context, exp explain.Explanation) (string, llm.Usage, error) {
	resp, err := t.LLM.Complete(ctx, llm.Request{
		Task: "story",
		Messages: []llm.Message{
			{Role: "system", Text: systemPrompt},
			{Role: "user", Text: Prompt(exp)},
		},
		MaxTokens:   MaxTokens,
		Temperature: 0.4,
	})
	if err != nil {
		return "", llm.Usage{}, err
	}
	text := strings.TrimSpace(resp.Text)
	if text == "" {
		return "", resp.Usage, ErrEmpty
	}
	return text, resp.Usage, nil
}

var typeNames = map[domain.NodeType]string{
	domain.NodeLaw:        "закон",
	domain.NodeQuantity:   "величина",
	domain.NodeFormula:    "формула",
	domain.NodePhenomenon: "явление",
}

// Prompt описывает цепочку для модели: цель, причинные цепочки и шаги с основаниями.
func Prompt(exp explain.Explanation) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Цель: «%s» (%s).\n", exp.Target.Title, typeNames[exp.Target.Type])
	titles := map[string]string{}
	for _, s := range exp.Steps {
		titles[s.Node.Slug] = s.Node.Title
	}
	for _, p := range exp.CausalPaths {
		names := make([]string, len(p))
		for i, s := range p {
			names[i] = titles[s]
		}
		fmt.Fprintf(&b, "Причинная цепочка явлений: %s.\n", strings.Join(names, " → "))
	}
	b.WriteString("\nШаги от оснований к цели:\n")
	for _, s := range exp.Steps {
		n := s.Node
		fmt.Fprintf(&b, "%d. %s «%s»", s.Index, typeNames[n.Type], n.Title)
		if n.Formula != "" {
			fmt.Fprintf(&b, ", формула $%s$", n.Formula)
		}
		if n.Unit != "" {
			fmt.Fprintf(&b, ", единица %s", n.Unit)
		}
		b.WriteString("\n")
		if n.Summary != "" {
			fmt.Fprintf(&b, "   Суть: %s\n", n.Summary)
		}
		for _, r := range s.Reasons {
			line := r.Text
			if r.Description != "" {
				line += " — " + r.Description
			}
			fmt.Fprintf(&b, "   Основание: %s\n", line)
		}
		if n.Warning != "" {
			fmt.Fprintf(&b, "   Ошибка в конспекте: %s\n", n.Warning)
		}
	}
	return b.String()
}
