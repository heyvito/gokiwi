package main

import (
	"os"
	"strings"

	"github.com/heyvito/gokiwi"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:        "gokiwi",
		HelpName:    "gokiwi",
		Usage:       "gokiwi [OPTIONS] schema-path",
		Version:     "0.0.1",
		Description: "Converts Kiwi schemas into Go source",
		Args:        true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "package",
				Usage:    "Package name to generate files",
				Aliases:  []string{"p"},
				Required: true,
			},
			&cli.StringSliceFlag{
				Name:    "extra",
				Aliases: []string{"e"},
				Usage:   "Adds an extra field to a given structure of message. Usage: StructName:FieldName:FieldType",
			},
			&cli.StringFlag{
				Name:     "output",
				Usage:    "Output path for generated sources",
				Required: true,
				Aliases:  []string{"o"},
			},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() == 0 {
				return cli.ShowAppHelp(c)
			}
			extraFields := map[string][]gokiwi.ExtraField{}
			for _, v := range c.StringSlice("extra") {
				values := strings.Split(v, ":")
				if len(values) != 3 {
					return cli.Exit("Invalid field definition '"+v+"': Expected format StructName:FieldName:FieldType", 1)
				}
				arr := extraFields[values[0]]
				arr = append(arr, gokiwi.ExtraField{
					TargetStruct: values[0],
					FieldName:    values[1],
					FieldType:    values[2],
				})
				extraFields[values[0]] = arr
			}

			path := c.Args().First()
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					return cli.Exit(path+": Not found", 1)
				}
				return err
			}

			schema, err := gokiwi.DecodeBinarySchema(data)
			if err != nil {
				return cli.Exit("Failed decoding schema: "+err.Error(), 1)
			}

			source := gokiwi.CompileGo(c.String("package"), schema, extraFields)
			err = os.WriteFile(c.String("output"), []byte(source), 0644)
			if err != nil {
				return cli.Exit("Failed writing output: "+err.Error(), 1)
			}

			return nil
		},
		Authors: []*cli.Author{{
			Name:  "Vito Sartori",
			Email: "hey@vito.io",
		}},
		Copyright: "Copyright (c) Vito Sartori",
	}

	if err := app.Run(os.Args); err != nil {
		panic(err)
	}
}
