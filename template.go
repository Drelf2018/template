package template

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"text/template"

	_ "unsafe"
)

// 环境
type Env map[string]any

func (e Env) Get(key string) func() any {
	return func() any { return e[key] }
}

//go:linkname GoodName text/template.goodName
func GoodName(name string) bool

// 保存环境变量
func (e Env) Set(tmpl *template.Template, prefix string) error {
	if e == nil {
		return nil
	}
	funcMap := make(template.FuncMap, len(e))
	for k := range e {
		if !GoodName(prefix + k) {
			return fmt.Errorf("template: invalid environment variable name: \"%s\"", prefix+k)
		}
		funcMap[prefix+k] = e.Get(k)
	}
	if len(funcMap) != 0 {
		tmpl.Funcs(funcMap)
	}
	return nil
}

// 模板
type Template struct {
	ID          uint64  `json:"-"           yaml:"-"         gorm:"primaryKey;autoIncrement"`  // 模板标识符
	Description string  `json:"description" yaml:"description"`                                // 模板介绍
	Author      string  `json:"author"      yaml:"author"    gorm:"index:idx_uses,priority:1"` // 模板作者
	Namespace   string  `json:"namespace"   yaml:"namespace" gorm:"index:idx_uses,priority:2"` // 模板命名
	Version     Version `json:"version"     yaml:"version"   gorm:"embedded"`                  // 模板版本
	Env         Env     `json:"env"         yaml:"env"       gorm:"serializer:json"`           // 模板环境
	Steps       []Step  `json:"steps"       yaml:"steps"     gorm:"foreignkey:TemplateID"`     // 模板步骤
}

func (t Template) String() string {
	if t.Description == "" {
		return fmt.Sprintf("%s/%s@%s", t.Author, t.Namespace, t.Version)
	}
	return fmt.Sprintf("%s/%s@%s: %s", t.Author, t.Namespace, t.Version, t.Description)
}

func (t Template) StringIndent(indent string) string {
	b := &strings.Builder{}
	write := func(pt *Template) { fmt.Fprintf(b, "%s/%s@%s", pt.Author, pt.Namespace, pt.Version) }
	write(&t)
	if len(t.Steps) != 0 {
		var f func(depth int, step Step)
		f = func(depth int, step Step) {
			b.WriteString("\n")
			for i := 0; i < depth; i++ {
				b.WriteString(indent)
			}
			if step.URL != "" {
				b.WriteString(step.Method)
				b.WriteByte(' ')
				b.WriteString(step.URL)
			} else {
				write(&step.Template)
			}
			for _, sub := range step.Steps {
				f(depth+1, sub)
			}
		}
		for _, step := range t.Steps {
			f(1, step)
		}
	}
	return b.String()
}

func (t *Template) Unmarshal(uses string) error {
	err := fmt.Errorf("template: invalid template uses: \"%s\", expected \"author/namespace@vX.Y.Z\"", uses)
	var ok bool
	t.Author, uses, ok = strings.Cut(uses, "/")
	if !ok {
		return err
	}
	t.Namespace, uses, ok = strings.Cut(uses, "@")
	if !ok {
		return err
	}
	return t.Version.UnmarshalText([]byte(uses))
}

func (t Template) Index() string {
	return fmt.Sprintf(
		`author = "%s" AND namespace = "%s" AND major = %d AND minor = %d AND patch = %d`,
		t.Author, t.Namespace, t.Version.Major, t.Version.Minor, t.Version.Patch,
	)
}

// 执行模板
func (t *Template) Do(tmpl *template.Template, data any) (out Env, result []byte, err error) {
	// 创建子模板避免环境变量跨域
	env, err := ToEnv(tmpl, "", data, t.Env)
	if err != nil {
		return nil, result, fmt.Errorf("template: failed to execute the environment variable \"%s\": %w", t, err)
	}
	tmpl = tmpl.New("")
	err = env.Set(tmpl, "")
	if err != nil {
		return nil, nil, err
	}
	out = make(Env)
	for _, step := range t.Steps {
		// 判断是否跳过步骤
		if step.Skip != "" {
			skip, err := ToString(tmpl, step.Skip, data)
			if err != nil {
				return nil, nil, fmt.Errorf("template: failed to execute step.Skip \"%s\": %w", step.Skip, err)
			}
			p, err := strconv.ParseBool(skip)
			if err != nil {
				return nil, nil, fmt.Errorf("template: failed to parse step.Skip \"%s\": %w", step.Skip, err)
			}
			if p {
				continue
			}
		}
		// 运行子模板
		if step.Uses != "" {
			// 没有初始化那就小小的帮你一下吧
			if len(step.Template.Steps) == 0 {
				err = Unmarshal(step.Uses, &step.Template)
				if err != nil {
					return nil, nil, fmt.Errorf("template: failed to unmarshal step.Uses \"%s\": %w", step.Uses, err)
				}
			}
			// 将子模板导出的变量作为这一步要设置的环境变量
			var o Env
			o, result, err = step.Template.Do(tmpl, data)
			if err != nil {
				return nil, nil, err
			}
			if step.Namespace != "" {
				err = o.Set(tmpl, step.Namespace+"_")
			} else {
				err = o.Set(tmpl, "")
			}
			if err != nil {
				return nil, nil, err
			}
		} else {
			// 请求三板斧
			req, err := step.Request(tmpl, data)
			if err != nil {
				return nil, nil, err
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return nil, nil, fmt.Errorf("template: failed to request step \"%s\": %w", step.Template, err)
			}
			// 读取响应结果
			result, err = io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return nil, result, fmt.Errorf("template: failed to read step \"%s\" response body: %w", step.Template, err)
			}
			// 检查响应状态码
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				resp.Body.Close()
				return nil, result, fmt.Errorf("template: failed to request step \"%s\" with status code: %d\n%s", step.Template, resp.StatusCode, string(result))
			}
		}
		// 设置环境变量
		text := fmt.Sprintf("{{ $response := %s }}", strconv.Quote(string(result)))
		for set := step.Set; set != nil; {
			v := set["set"]
			delete(set, "set")
			parent, err := ToEnv(tmpl, text, data, set)
			if err != nil {
				return nil, result, fmt.Errorf("template: failed to execute the environment variable \"%s\": %w", step.Template, err)
			}
			err = parent.Set(tmpl, "")
			if err != nil {
				return nil, nil, err
			}
			switch v := v.(type) {
			case map[string]any:
				set = v
			case Env:
				set = v
			case nil:
				set = nil
			default:
				return nil, nil, fmt.Errorf("template: invalid environment variable value: \"%v\"", v)
			}
		}
		// 筛选出需要导出的变量
		if len(step.Out) != 0 {
			export, err := ToEnv(tmpl, text, data, step.Out)
			if err != nil {
				return nil, result, fmt.Errorf("template: failed to execute the output variable \"%s\": %w", step.Template, err)
			}
			for k, v := range export {
				out[k] = v
			}
		}
	}
	return
}

// 步骤
type Step struct {
	Template   `yaml:",inline"`
	TemplateID uint64      `json:"-"       yaml:"-"`                             // 模板外键
	Skip       string      `json:"skip"    yaml:"skip"`                          // 跳过步骤
	Uses       string      `json:"uses"    yaml:"uses"`                          // 使用模板
	Method     string      `json:"method"  yaml:"method"`                        // 请求方法
	URL        string      `json:"url"     yaml:"url"`                           // 请求地址
	Body       string      `json:"body"    yaml:"body"`                          // 请求内容
	Header     http.Header `json:"header"  yaml:"header" gorm:"serializer:json"` // 请求头部
	Set        Env         `json:"set"     yaml:"set"    gorm:"serializer:json"` // 设置环境变量
	Out        Env         `json:"out"     yaml:"out"    gorm:"serializer:json"` // 导出环境变量
}

var ErrEmptyURL = errors.New("template: step URL is empty")

// 步骤转请求
func (s *Step) Request(tmpl *template.Template, data any) (*http.Request, error) {
	// 解析链接
	if s.URL == "" {
		return nil, ErrEmptyURL
	}
	url, err := ToString(tmpl, s.URL, data)
	if err != nil {
		return nil, fmt.Errorf("template: execute step.URL \"%s\" failed: %w", s.URL, err)
	}
	// 解析请求体
	var body io.Reader
	if s.Body != "" {
		body, err = ToBuffer(tmpl, s.Body, data)
		if err != nil {
			return nil, fmt.Errorf("template: execute step.Body \"%s\" failed: %w", s.Body, err)
		}
	}
	// 创建请求
	req, err := http.NewRequest(s.Method, url, body)
	if err != nil {
		return nil, fmt.Errorf("template: create step \"%s\" request failed: %w", s.Template, err)
	}
	// 解析请求头
	for k, vs := range s.Header {
		for _, v := range vs {
			v, err = ToString(tmpl, v, data)
			if err != nil {
				return nil, fmt.Errorf("template: execute step.Header \"%s\" failed: %w", k, err)
			}
			req.Header.Add(k, v)
		}
	}
	return req, nil
}
