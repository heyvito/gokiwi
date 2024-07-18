package gokiwi

import (
	"fmt"
	"go/format"
	"strings"
	"unicode"

	"github.com/stoewer/go-strcase"
)

type ExtraField struct {
	TargetStruct string
	FieldName    string
	FieldType    string
}

type sbuilder struct {
	strings.Builder
}

func (s *sbuilder) Writef(format string, args ...interface{}) {
	s.WriteString(fmt.Sprintf(format, args...))
}

type goCompiler struct {
	enums   map[string]bool
	structs map[string]bool
	schema  *Schema
}

func maybeCamel(s string) string {
	if unicode.IsUpper(rune(s[0])) {
		return s
	}
	return strcase.UpperCamelCase(s)
}

func (g *goCompiler) compileMessage(s *sbuilder, d *Definition, fields map[string][]ExtraField) {
	s.Writef("type %s struct {\n", d.Name)
	for _, field := range d.Fields {
		fieldType := *field.Type
		if fieldType == "float" {
			fieldType = "float64"
		}
		arrayNotation := ""
		if field.IsArray {
			arrayNotation = "[]"
		}
		if unicode.IsUpper(rune(fieldType[0])) && !g.isEnum(string(fieldType)) {
			fieldType = "*" + fieldType
		}
		s.Writef("%s %s%s `kiwi_index:\"%d\"`\n", maybeCamel(field.Name), arrayNotation, fieldType, field.Value)
	}

	for _, field := range fields[d.Name] {
		s.Writef("%s %s\n", field.FieldName, field.FieldType)
	}

	s.Writef("}\n\n")
}

func (g *goCompiler) compileMessageDecoder(b *sbuilder, d *Definition) {
	b.Writef("func Decode%s(b *gokiwi.Buffer) (res *%s, err error) {\n", maybeCamel(d.Name), d.Name)
	b.Writef("res = &%s{}\n", d.Name)
	b.Writef("var idx uint\n")
	b.Writef("\n")
	b.Writef("loop:")
	b.Writef("for {\n")
	b.Writef("idx, err = b.ReadVarUint()\n")
	b.Writef("if err != nil { return nil, err }\n")
	b.Writef("switch idx {\n")
	b.Writef("case 0:\n")
	b.Writef("break loop\n")

	for _, field := range d.Fields {
		b.Writef("case %d:\n", field.Value)
		if field.IsArray {
			g.processArrayField(b, field)
		} else {
			g.processSimpleField(b, field)
		}
	}

	b.Writef("}\n")
	b.Writef("}\n")

	b.Writef("return")
	b.Writef("}\n\n")
}

func (g *goCompiler) compileStruct(b *sbuilder, d *Definition, fields map[string][]ExtraField) {
	b.Writef("type %s struct {\n", d.Name)
	for _, field := range d.Fields {
		fieldType := *field.Type
		if fieldType == "float" {
			fieldType = "float64"
		}
		arrayNotation := ""
		if field.IsArray {
			arrayNotation = "[]"
		}
		if unicode.IsUpper(rune(fieldType[0])) && !g.isEnum(string(fieldType)) {
			fieldType = "*" + fieldType
		}
		b.Writef("%s %s%s\n", maybeCamel(field.Name), arrayNotation, fieldType)
	}
	for _, field := range fields[d.Name] {
		b.Writef("%s %s\n", field.FieldName, field.FieldType)
	}
	b.Writef("}\n\n")
}

func (g *goCompiler) compileStructDecoder(b *sbuilder, d *Definition) {
	b.Writef("func Decode%s(b *gokiwi.Buffer) (res *%s, err error) {\n", maybeCamel(d.Name), d.Name)
	b.Writef("res = &%s{}\n", d.Name)
	for _, field := range d.Fields {
		if field.IsArray {
			g.processArrayField(b, field)
		} else {
			g.processSimpleField(b, field)
		}
	}

	b.Writef("return")
	b.Writef("}\n\n")
}

func (g *goCompiler) compileEnum(buf *sbuilder, schema *Definition) {
	buf.Writef("type %s int\n", schema.Name)
	buf.Writef("const (\n")
	for _, v := range schema.Fields {
		buf.Writef("%s%s %s = %d\n", schema.Name, strcase.UpperCamelCase(v.Name), schema.Name, v.Value)
	}
	buf.Writef(")\n")
}

func (g *goCompiler) isEnum(name string) bool {
	_, ok := g.enums[name]
	return ok
}

func (g *goCompiler) processSimpleField(b *sbuilder, field Field) {
	isSimple := true
	decoder := ""
	switch *field.Type {
	case Bool:
		b.Writef("v, err := b.ReadByte()\n")
		b.Writef("if err != nil { return nil, err }\n")
		b.Writef("res.%s = v != 0\n", maybeCamel(field.Name))
		isSimple = false
	case Byte:
		decoder = "ReadByte"
	case Int:
		decoder = "ReadVarInt"
	case Uint:
		decoder = "ReadVarUint"
	case Float:
		decoder = "ReadVarFloat"
	case String:
		decoder = "ReadString"
	case Int64:
		decoder = "ReadVarInt64"
	case Uint64:
		decoder = "ReadVarUint64"
	default:
		isSimple = false
		if g.isEnum(string(*field.Type)) {
			b.Writef("v, err := b.ReadVarUint()\n")
			b.Writef("if err != nil { return nil, err }\n")
			b.Writef("res.%s = %s(v)\n", maybeCamel(field.Name), *field.Type)
		} else {
			b.Writef("res.%s, err = Decode%s(b)\n", maybeCamel(field.Name), *field.Type)
			b.Writef("if err != nil { return nil, err }\n")
		}
	}

	if isSimple {
		b.Writef("res.%s, err = b.%s()\n", maybeCamel(field.Name), decoder)
		b.Writef("if err != nil { return nil, err }\n")
	}
}

func (g *goCompiler) processArrayField(b *sbuilder, field Field) {
	b.Writef("{\n")
	b.Writef("size, err := b.ReadVarUint()\n")
	b.Writef("if err != nil { return nil, err }\n")
	b.Writef("values := make([]")
	if t, ok := goTypes[*field.Type]; ok {
		b.Writef(t)
	} else if g.isEnum(string(*field.Type)) {
		b.Writef(string(*field.Type))
	} else {
		b.Writef("*%s", maybeCamel(string(*field.Type)))
	}
	b.Writef(", size)\n")
	b.Writef("for i := range size {\n")
	if _, ok := goTypes[*field.Type]; ok {
		simple := true
		switch *field.Type {
		case Bool:
			simple = false
			b.Writef("rawByte, err := b.ReadByte()\n")
			b.Writef("if err != nil { return nil, err }\n")
			b.Writef("values[i] = rawByte != 0\n")
		case Byte:
			b.Writef("values[i], err = b.ReadByte()\n")
		case Int:
			b.Writef("values[i], err = b.ReadVarInt()\n")
		case Uint:
			b.Writef("values[i], err = b.ReadVarUint()\n")
		case Float:
			b.Writef("values[i], err = b.ReadVarFloat()\n")
		case String:
			b.Writef("values[i], err = b.ReadString()\n")
		case Int64:
			b.Writef("values[i], err = b.ReadVarInt64()\n")
		case Uint64:
			b.Writef("values[i], err = b.ReadVarUint64()\n")
		}
		if simple {
			b.Writef("if err != nil { return nil, err }\n")
		}

	} else if g.isEnum(string(*field.Type)) {
		b.Writef("v, err := b.ReadVarUint()\n")
		b.Writef("if err != nil { return nil, err }\n")
		b.Writef("values[i] = %s(v)\n", *field.Type)
	} else {
		b.Writef("v, err := Decode%s(b)\n", *field.Type)
		b.Writef("if err != nil { return nil, err }\n")
		b.Writef("values[i] = v\n")
	}
	b.Writef("}\n")
	b.Writef("res.%s = values\n", maybeCamel(field.Name))
	b.Writef("}\n")
}

func CompileGo(packageName string, schema *Schema, fields map[string][]ExtraField) string {
	comp := goCompiler{
		enums:   map[string]bool{},
		structs: map[string]bool{},
		schema:  schema,
	}

	for _, v := range schema.Definitions {
		if v.Kind == ENUM {
			comp.enums[v.Name] = true
		} else {
			comp.structs[v.Name] = true
		}
	}

	buf := sbuilder{}
	buf.Writef("// Code generated by github.com/heyvito/gokiwi. DO NOT EDIT.\n\n")
	buf.Writef("package %s\n\n", packageName)
	buf.Writef(`import "github.com/heyvito/gokiwi"` + "\n\n")
	for _, v := range schema.Definitions {
		switch v.Kind {
		case ENUM:
			comp.compileEnum(&buf, &v)
		case MESSAGE:
			comp.compileMessage(&buf, &v, fields)
			comp.compileMessageDecoder(&buf, &v)
		case STRUCT:
			comp.compileStruct(&buf, &v, fields)
			comp.compileStructDecoder(&buf, &v)
		}
	}

	s, err := format.Source([]byte(buf.String()))
	if err != nil {
		panic(err)
	}

	return string(s)
}
