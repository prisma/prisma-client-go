package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/prisma/photongo/generator/runtime"
	"github.com/prisma/photongo/logger"
)

type Input struct {
	Name   string
	Fields []Field
	Value  interface{}
}

// Output can be a single Name or can have nested fields
type Output struct {
	Name string

	// Inputs (optional) to provide arguments to a field
	Inputs []Input

	Outputs []Output
}

type Field struct {
	// The Name of the field.
	Name string

	// an Action for input fields, e.g. `contains`
	Action string

	// List saves whether the fields is a list of items
	List bool

	// WrapList saves whether the a list field should be wrapped in an object
	WrapList bool

	// Value contains the field value. if nil, fields will contain a subselection.
	Value interface{}

	// Fields contains a subselection of fields. If not nil, value will be undefined.
	Fields []Field
}

type Client interface {
	Do(ctx context.Context, query string, into interface{}) error
}

type Query struct {
	// Client is the generic Photon Client
	Client Client

	// Operation describes the PQL operation: query, mutation or subscription
	Operation string

	// Name describes the operation; useful for tracing
	Name string

	// Method describes a crud operation
	Method string

	// Model contains the Prisma model Name
	Model string

	// Inputs contains function arguments
	Inputs []Input

	// Outputs contains the return fields
	Outputs []Output
}

func (q Query) buildQuery() string {
	var builder strings.Builder

	builder.WriteString(q.Operation + " " + q.Name)
	builder.WriteString("{")

	builder.WriteString(q.Build())

	builder.WriteString("}")

	return builder.String()
}

func (q Query) Build() string {
	var builder strings.Builder

	builder.WriteString(q.Method + q.Model)

	if len(q.Inputs) > 0 {
		builder.WriteString(q.buildInputs(q.Inputs))
	}

	builder.WriteString(" ")

	builder.WriteString(q.buildOutputs(q.Outputs))

	return builder.String()
}

func (q Query) buildInputs(inputs []Input) string {
	var builder strings.Builder

	builder.WriteString("(")

	for _, i := range inputs {
		builder.WriteString(i.Name)

		builder.WriteString(":")

		if i.Value != nil {
			builder.Write(value(i.Value))
		} else {
			builder.WriteString(q.buildFields(false, false, i.Fields))
		}

		builder.WriteString(",")
	}

	builder.WriteString(")")

	return builder.String()
}

func (q Query) buildOutputs(outputs []Output) string {
	var builder strings.Builder

	builder.WriteString("{")

	for _, o := range outputs {
		builder.WriteString(o.Name + " ")

		if len(o.Inputs) > 0 {
			log.Printf("building inputs: %d %+v", len(o.Inputs), o.Inputs)
			builder.WriteString(q.buildInputs(o.Inputs))
		}

		if len(o.Outputs) > 0 {
			builder.WriteString(q.buildOutputs(o.Outputs))
		}
	}

	builder.WriteString("}")

	return builder.String()
}

func (q Query) buildFields(list bool, wrapList bool, fields []Field) string {
	var builder strings.Builder

	if !list {
		builder.WriteString("{")
	}

	for _, f := range fields {
		if wrapList {
			builder.WriteString("{")
		}

		if f.Name != "" {
			builder.WriteString(f.Name)
		}

		if f.Name != "" && f.Action != "" {
			builder.WriteString("_" + f.Action)
		}

		if f.Name != "" {
			builder.WriteString(":")
		}

		if f.List {
			builder.WriteString("[")
		}

		if f.Fields != nil {
			builder.WriteString(q.buildFields(f.List, f.WrapList, f.Fields))
		}

		if f.Value != nil {
			builder.Write(value(f.Value))
		}

		if f.List {
			builder.WriteString("]")
		}

		if wrapList {
			builder.WriteString("}")
		}

		builder.WriteString(",")
	}

	if !list {
		builder.WriteString("}")
	}

	return builder.String()
}

func (q Query) Exec(ctx context.Context, v interface{}) error {
	if q.Client == nil {
		panic("client.Connect() needs to be called before sending queries")
	}

	s := q.buildQuery()

	// TODO use specific log level
	if logger.Enabled {
		logger.Debug.Printf("prisma query: `%s`", s)
	}

	return q.Client.Do(ctx, s, &v)
}

func value(value interface{}) []byte {
	if v, ok := value.(time.Time); ok {
		return []byte(fmt.Sprintf(`"%s"`, v.UTC().Format(runtime.RFC3339Milli)))
	}

	v, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}

	return v
}
