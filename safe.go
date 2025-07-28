package template

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"text/template"

	"github.com/PuerkitoBio/goquery"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

// 获取纯净文本
func Plaintext(text string) (string, error) {
	html := strings.NewReader(text)
	doc, err := goquery.NewDocumentFromReader(html)
	if err != nil {
		return "", err
	}
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		if alt, ok := s.Attr("alt"); ok {
			s.SetText(alt)
		}
	})
	return doc.Text(), nil
}

// 内置函数
var BuiltinFuncMap = template.FuncMap{
	"json": func(v any) (string, error) {
		b, err := json.Marshal(v)
		return string(b), err
	},
	"gjson": func(json, path string) any {
		return gjson.Get(json, path).Value()
	},
	"gjson2": func(json, path string) (any, error) {
		if r := gjson.Get(json, path); r.Exists() {
			return r.Value(), nil
		}
		return nil, fmt.Errorf("path not found: \"%s\"", path)
	},
	"base64encode": func(s string) string {
		return base64.StdEncoding.EncodeToString([]byte(s))
	},
	"base64decode": func(s string) (string, error) {
		data, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return "", err
		}
		return string(data), nil
	},
	"plaintext": Plaintext,
}

// 并发安全模板
type SafeTemplate struct {
	data reflect.Value
	root *template.Template
}

// 运行模板
func (st *SafeTemplate) Do(rt *Template) (out Env, result []byte, err error) {
	return rt.Do(st.root.New(uuid.NewString()), st.data)
}

func NewSafeTemplate(v any) *SafeTemplate {
	return &SafeTemplate{
		data: reflect.ValueOf(v),
		root: template.New("root").Funcs(BuiltinFuncMap),
	}
}
