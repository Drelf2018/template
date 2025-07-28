package template

import (
	"bytes"
	"text/template"
)

// 模板转缓冲器
func ToBuffer(tmpl *template.Template, text string, data any) (*bytes.Buffer, error) {
	tmpl, err := tmpl.Parse(text)
	if err != nil {
		return nil, err
	}
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, data)
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// 模板转文本
func ToString(tmpl *template.Template, text string, data any) (string, error) {
	buf, err := ToBuffer(tmpl, text, data)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// 模板转变量
func ToEnv(tmpl *template.Template, text string, data any, env Env) (out Env, err error) {
	out = make(Env, len(env))
	for k, v := range env {
		if s, ok := v.(string); ok {
			out[k], err = ToString(tmpl, text+s, data)
			if err != nil {
				return
			}
		} else {
			out[k] = v
		}
	}
	return
}
