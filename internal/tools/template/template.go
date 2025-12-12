package template

import (
	"bytes"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

type TemplateValue struct {
	Key   string
	Value string
}

func FromTemplateValues(tplValues []TemplateValue) map[string]any {
	values := make(map[string]any)
	for _, v := range tplValues {
		values[v.Key] = v.Value
	}
	return values
}

type Template string

func (t Template) Render(values map[string]any) ([]byte, error) {
	tpl, err := template.New("template").Funcs(sprig.FuncMap()).Parse(string(t))
	if err != nil {
		return nil, err
	}

	buf := bytes.Buffer{}
	if err := tpl.Execute(&buf, values); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
