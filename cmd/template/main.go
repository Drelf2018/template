package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Drelf2018/template"
)

var ErrEmptyTemplatePath = errors.New("template/cmd/template: empty template path")

func init() {
	template.Decoder = template.FileDecoder{}
}

func main() {
	if len(os.Args) < 2 {
		panic(ErrEmptyTemplatePath)
	}
	tmpl := os.Args[1]
	var t template.Template
	err := template.Unmarshal(tmpl, &t)
	if err != nil {
		panic(err)
	}
	// 解析数据
	for _, expr := range os.Args[2:] {
		k, v, ok := strings.Cut(expr, "=")
		if !ok {
			continue
		}
		t.Env[strings.TrimPrefix(k, "--")] = v
	}
	// 运行模板
	out, _, err := template.NewSafeTemplate(nil).Do(&t)
	if err != nil {
		panic(err)
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
}
