package stdjson

import (
	"encoding/json"
	"io"
	"os"

	"github.com/Sn0wo2/go-common/helper"
)

// Pretty controls indented (human-readable) JSON output.
var Pretty bool

type Stage string

const (
	StageInfo   Stage = "info"
	StageWarn   Stage = "warn"
	StageError  Stage = "error"
	StageResult Stage = "result"
)

type OutPut[T any] struct {
	Stage `json:"status"`
	Msg   string `json:"msg"`
	Data  T      `json:"data,omitempty"`
}

func New(status Stage, msg string) *OutPut[any] {
	return &OutPut[any]{
		Stage: status,
		Msg:   msg,
	}
}

func (o *OutPut[T]) WithData(data T) *OutPut[T] {
	o.Data = data
	return o
}

func (o *OutPut[T]) Bytes() []byte {
	var output []byte
	var err error
	if Pretty {
		output, err = json.MarshalIndent(o, "", "  ")
	} else {
		output, err = json.Marshal(o)
	}
	if err != nil {
		return nil
	}
	return output
}

func (o *OutPut[T]) String() string {
	return helper.BytesToString(o.Bytes())
}

func (o *OutPut[T]) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(append(o.Bytes(), '\n'))
	return int64(n), err
}

func (o *OutPut[T]) Write() (int64, error) {
	switch o.Stage {
	case StageWarn, StageError:
		return o.WriteTo(os.Stderr)
	default:
		return o.WriteTo(os.Stdout)
	}
}
