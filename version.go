package template

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	ErrEmptyInput     = errors.New("template: empty input")
	ErrNotString      = errors.New("template: version must be a string")
	ErrInvalidFormat  = errors.New("template: invalid version format: expected vX.Y.Z")
	ErrEmptyComponent = errors.New("template: empty version component")
)

// 版本号
type Version struct {
	// 主版本号
	// 这个数字的变化表示了一个重大更新，通常伴随着不兼容的API变更或者重大的功能改进
	Major uint64 `gorm:"index:idx_uses,priority:3"`

	// 次版本号
	// 这个数字的变化表示了向后兼容的新功能添加，通常是对现有功能的扩展
	Minor uint64 `gorm:"index:idx_uses,priority:4"`

	// 修订号
	// 这个数字的变化表示了向后兼容的问题修复，通常是对现有功能的错误修正
	Patch uint64 `gorm:"index:idx_uses,priority:5"`
}

func (v Version) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v Version) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}

func (v *Version) UnmarshalText(data []byte) error {
	if len(data) == 0 {
		return ErrEmptyInput
	}
	if data[0] != 'v' {
		return fmt.Errorf("template: invalid prefix '%c'", data[0])
	}
	// 去除'v'前缀
	s := string(data[1:])
	// 分割版本号
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return ErrInvalidFormat
	}
	// 解析整数的函数
	parsePart := func(part string) (uint64, error) {
		clean := strings.TrimSpace(part)
		if clean == "" {
			return 0, ErrEmptyComponent
		}
		val, err := strconv.ParseUint(clean, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("template: invalid number '%s': %w", clean, err)
		}
		return val, nil
	}
	// 解析版本数字
	major, err := parsePart(parts[0])
	if err != nil {
		return err
	}
	minor, err := parsePart(parts[1])
	if err != nil {
		return err
	}
	patch, err := parsePart(parts[2])
	if err != nil {
		return err
	}
	// 赋值
	v.Major = major
	v.Minor = minor
	v.Patch = patch
	return nil
}

func (v Version) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"v%d.%d.%d\"", v.Major, v.Minor, v.Patch)), nil
}

func (v *Version) UnmarshalJSON(b []byte) error {
	if len(b) <= 2 {
		return ErrEmptyInput
	}
	// 去除 JSON 引号
	if b[0] != '"' || b[len(b)-1] != '"' {
		return ErrNotString
	}
	return v.UnmarshalText(b[1 : len(b)-1])
}

func (v Version) MarshalYAML() (any, error) {
	return v.String(), nil
}

func (v *Version) UnmarshalYAML(value *yaml.Node) error {
	// 确保 YAML 节点是字符串类型
	if value.Tag != "!!str" {
		return ErrNotString
	}
	return v.UnmarshalText([]byte(value.Value))
}

func (v Version) Equal(other Version) bool {
	return v.Major == other.Major && v.Minor == other.Minor && v.Patch == other.Patch
}

func (v Version) Less(other Version) bool {
	if v.Major < other.Major {
		return true
	}
	if v.Major > other.Major {
		return false
	}
	if v.Minor < other.Minor {
		return true
	}
	if v.Minor > other.Minor {
		return false
	}
	return v.Patch < other.Patch
}

func (v Version) IsZero() bool {
	return v.Major == 0 && v.Minor == 0 && v.Patch == 0
}

var _ interface {
	encoding.TextMarshaler
	encoding.TextUnmarshaler
	json.Marshaler
	json.Unmarshaler
	yaml.Marshaler
	yaml.Unmarshaler
} = (*Version)(nil)
