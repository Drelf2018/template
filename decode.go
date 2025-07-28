package template

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

type Unmarshaler interface {
	UnmarshalTemplate(uses string, tmpl *Template) error
}

var Decoder Unmarshaler

// 文件解码器
// 可以从 JSON 和 YAML 文件中解析模板
type FileDecoder struct{}

func (FileDecoder) UnmarshalTemplate(uses string, tmpl *Template) error {
	b, err := os.ReadFile(uses)
	if err != nil {
		return err
	}
	ext := filepath.Ext(uses)
	switch strings.ToLower(ext) {
	case ".json":
		err = json.Unmarshal(b, tmpl)
	case ".yml", ".yaml":
		err = yaml.Unmarshal(b, tmpl)
	default:
		err = fmt.Errorf("template: invalid type: %s", ext)
	}
	return err
}

var _ Unmarshaler = (*FileDecoder)(nil)

// 数据库解码器
// 可以从数据库 steps 表中解析模板
type DatabaseDecoder struct {
	*gorm.DB
}

func (d *DatabaseDecoder) UnmarshalTemplate(uses string, tmpl *Template) error {
	err := tmpl.Unmarshal(uses)
	if err != nil {
		return err
	}
	if d.DB.Model(&Step{}).Preload("Steps").Limit(1).Find(tmpl, tmpl.Index()).RowsAffected == 0 {
		return fmt.Errorf("template: template uses not found: \"%s\"", uses)
	}
	return nil
}

var _ Unmarshaler = (*DatabaseDecoder)(nil)

// 附加环境
func AppendEnv(e Env, elems ...Env) Env {
	n := make(Env, len(e))
	for k, v := range e {
		n[k] = v
	}
	for _, elem := range elems {
		for k, v := range elem {
			n[k] = v
		}
	}
	return n
}

var ErrInvalidDecoder = errors.New("template: invalid Decoder")

// 递归解析子模板
func Unmarshal(uses string, tmpl *Template) error {
	if Decoder == nil {
		return ErrInvalidDecoder
	}
	env := AppendEnv(tmpl.Env)
	namespace := tmpl.Namespace
	err := Decoder.UnmarshalTemplate(uses, tmpl)
	if err != nil {
		return err
	}
	if namespace != "" {
		tmpl.Namespace = namespace
	}
	tmpl.Env = AppendEnv(tmpl.Env, env)
	for i := range tmpl.Steps {
		if tmpl.Steps[i].Uses != "" {
			err = Unmarshal(tmpl.Steps[i].Uses, &tmpl.Steps[i].Template)
			if err != nil {
				return err
			}
		}
		if tmpl.Steps[i].Out == nil {
			tmpl.Steps[i].Out = Env{}
		}
	}
	return nil
}
