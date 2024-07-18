package gokiwi

import "fmt"

type Type string

const (
	Bool   Type = "bool"
	Byte   Type = "byte"
	Int    Type = "int"
	Uint   Type = "uint"
	Float  Type = "float"
	String Type = "string"
	Int64  Type = "int64"
	Uint64 Type = "uint64"
)

var allTypes = []Type{Bool, Byte, Int, Uint, Float, String, Int64, Uint64}
var goTypes = map[Type]string{
	Bool:   "bool",
	Byte:   "byte",
	Int:    "int",
	Uint:   "uint",
	Float:  "float64",
	String: "string",
	Int64:  "int64",
	Uint64: "uint64",
}

type DefinitionKind string

const (
	ENUM    DefinitionKind = "ENUM"
	STRUCT  DefinitionKind = "STRUCT"
	MESSAGE DefinitionKind = "MESSAGE"
)

var allDefinitionKinds = []DefinitionKind{ENUM, STRUCT, MESSAGE}

type Schema struct {
	Package     *string
	Definitions []Definition
}

type Definition struct {
	Name   string
	Line   int
	Column int
	Kind   DefinitionKind
	Fields []Field
}

type Field struct {
	Name         string
	Line         int
	Column       int
	RawType      *int
	Type         *Type
	IsArray      bool
	IsDeprecated bool
	Value        uint
}

func DecodeBinarySchema(data []byte) (*Schema, error) {
	bb := Buffer{data: data, length: len(data)}
	rawDefCount, err := bb.ReadVarUint()
	if err != nil {
		return nil, err
	}
	defCount := int(rawDefCount)
	defs := make([]Definition, 0, defCount)

	for i := 0; i < defCount; i++ {
		defName, err := bb.ReadString()
		if err != nil {
			return nil, err
		}
		kind, err := bb.ReadByte()
		if err != nil {
			return nil, err
		}
		fieldCount, err := bb.ReadVarUint()
		if err != nil {
			return nil, err
		}
		fields := make([]Field, 0, fieldCount)

		for j := 0; j < int(fieldCount); j++ {
			fieldName, err := bb.ReadString()
			if err != nil {
				return nil, err
			}
			fieldType, err := bb.ReadVarInt()
			if err != nil {
				return nil, err
			}
			arrayFlag, err := bb.ReadByte()
			if err != nil {
				return nil, err
			}
			isArray := (arrayFlag & 1) == 1
			value, err := bb.ReadVarUint()
			if err != nil {
				return nil, err
			}
			f := Field{
				Name:         fieldName,
				Line:         0,
				Column:       0,
				RawType:      nil,
				IsArray:      isArray,
				IsDeprecated: false,
				Value:        value,
			}
			if allDefinitionKinds[kind] != ENUM {
				f.RawType = &fieldType
			}
			fields = append(fields, f)
		}

		defs = append(defs, Definition{
			Name:   defName,
			Line:   0,
			Column: 0,
			Kind:   allDefinitionKinds[kind],
			Fields: fields,
		})
	}

	for i := 0; i < defCount; i++ {
		fields := defs[i].Fields
		for j, field := range fields {
			rawType := field.RawType
			if rawType != nil && *rawType < 0 {
				if ^*rawType >= len(allTypes) {
					panic(fmt.Sprintf("Invalid type %d", *rawType))
				}
				field.Type = &allTypes[^*rawType]
			} else {
				if rawType != nil && *rawType >= len(defs) {
					panic(fmt.Sprintf("Invalid type %d", *rawType))
				}
				if rawType == nil {
					field.Type = nil
				} else {
					theType := Type(defs[*rawType].Name)
					field.Type = &theType
				}
			}
			fields[j] = field
		}
	}

	return &Schema{
		Package:     nil,
		Definitions: defs,
	}, nil
}
